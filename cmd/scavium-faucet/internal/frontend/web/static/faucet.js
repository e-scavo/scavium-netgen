(function () {
  var state = {
    explorerTxURL: "",
    pollTimer: null,
    currentClaimID: "",
    statusStopSet: { confirmed: true, failed: true, rejected: true },
    captchaProvider: "disabled",
    captchaSiteKey: "",
    captchaToken: "",
    captchaWidgetID: null,
    tokens: [],
    selectedTokenID: "",
    tokenCatalogStatus: "loading",
    explorerURLSafe: false,
  };

  function el(id) {
    return document.getElementById(id);
  }

  function setHidden(node, hidden) {
    if (!node) {
      return;
    }
    if (hidden) {
      node.classList.add("hidden");
    } else {
      node.classList.remove("hidden");
    }
  }

  function setStatus(node, text, kind) {
    if (!node) {
      return;
    }
    if (!text) {
      node.className = "status";
      node.textContent = "";
      node.setAttribute("aria-hidden", "true");
      return;
    }
    node.className = "status visible " + (kind || "info");
    node.textContent = text;
    node.removeAttribute("aria-hidden");
  }

  function setBusy(button, busy, text) {
    if (!button) {
      return;
    }
    if (busy) {
      button.dataset.defaultText = button.dataset.defaultText || button.textContent;
      button.textContent = text || "Working...";
      button.disabled = true;
      button.setAttribute("aria-busy", "true");
      return;
    }
    button.textContent = button.dataset.defaultText || button.textContent;
    button.disabled = false;
    button.removeAttribute("aria-busy");
  }

  function isAbsoluteHTTPURLTemplate(template) {
    return /^https?:\/\//i.test(String(template || ""));
  }

  function validExplorerTemplate(template) {
    if (!template || template.indexOf("{txHash}") === -1 || !isAbsoluteHTTPURLTemplate(template)) {
      return false;
    }
    try {
      var probe = new URL(template.replace("{txHash}", "0x" + "0".repeat(64)));
      return probe.protocol === "https:" || probe.protocol === "http:";
    } catch (_) {
      return false;
    }
  }

  function txExplorerHref(txHash) {
    var hash = String(txHash || "");
    if (!/^0x[0-9a-fA-F]{64}$/.test(hash) || !state.explorerURLSafe) {
      return "";
    }
    try {
      var href = state.explorerTxURL.replace("{txHash}", encodeURIComponent(hash));
      var url = new URL(href);
      if ((url.protocol !== "https:" && url.protocol !== "http:") || !isAbsoluteHTTPURLTemplate(href)) {
        return "";
      }
      return url.href;
    } catch (_) {
      return "";
    }
  }

  function currentAddress() {
    return (el("address") && el("address").value ? el("address").value.trim() : "");
  }

  function resetCaptchaToken() {
    state.captchaToken = "";
    var tokenInput = el("captcha-token");
    if (tokenInput) {
      tokenInput.value = "";
    }
  }

  function loadScriptOnce(id, src) {
    return new Promise(function (resolve, reject) {
      if (document.getElementById(id)) {
        resolve();
        return;
      }
      var script = document.createElement("script");
      script.id = id;
      script.src = src;
      script.async = true;
      script.defer = true;
      script.onload = function () { resolve(); };
      script.onerror = function () { reject(new Error("captcha script load failed")); };
      document.head.appendChild(script);
    });
  }

  function setCaptchaToken(token) {
    state.captchaToken = token || "";
    var tokenInput = el("captcha-token");
    if (tokenInput) {
      tokenInput.value = state.captchaToken;
    }
  }

  async function setupCaptcha(provider, siteKey) {
    state.captchaProvider = String(provider || "disabled").toLowerCase();
    state.captchaSiteKey = siteKey || "";
    resetCaptchaToken();

    var row = el("captcha-row");
    var widget = el("captcha-widget");
    var manual = el("captcha-manual");
    setHidden(row, !(state.captchaProvider && state.captchaProvider !== "disabled"));
    setHidden(manual, true);
    if (!widget) {
      return;
    }
    widget.innerHTML = "";

    if (!state.captchaProvider || state.captchaProvider === "disabled") {
      return;
    }
    if (state.captchaProvider === "dev") {
      setHidden(manual, false);
      setCaptchaToken("dev-bypass");
      return;
    }
    if (!state.captchaSiteKey) {
      setHidden(manual, false);
      setStatus(el("msg"), "Captcha is enabled but no public site key was provided.", "warn");
      return;
    }

    try {
      if (state.captchaProvider === "turnstile") {
        await loadScriptOnce("turnstile-api", "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit");
        if (window.turnstile && typeof window.turnstile.render === "function") {
          state.captchaWidgetID = window.turnstile.render(widget, {
            sitekey: state.captchaSiteKey,
            callback: setCaptchaToken,
            "expired-callback": resetCaptchaToken,
            "error-callback": resetCaptchaToken,
          });
          return;
        }
      }
      if (state.captchaProvider === "hcaptcha") {
        await loadScriptOnce("hcaptcha-api", "https://js.hcaptcha.com/1/api.js?render=explicit");
        if (window.hcaptcha && typeof window.hcaptcha.render === "function") {
          state.captchaWidgetID = window.hcaptcha.render(widget, {
            sitekey: state.captchaSiteKey,
            callback: setCaptchaToken,
            "expired-callback": resetCaptchaToken,
            "error-callback": resetCaptchaToken,
          });
          return;
        }
      }
      setHidden(manual, false);
    } catch (_) {
      setHidden(manual, false);
      setStatus(el("msg"), "Captcha widget could not be loaded. Retry or use a supported browser.", "warn");
    }
  }

  function refreshCaptcha() {
    resetCaptchaToken();
    if (state.captchaProvider === "turnstile" && window.turnstile && state.captchaWidgetID != null) {
      window.turnstile.reset(state.captchaWidgetID);
    }
    if (state.captchaProvider === "hcaptcha" && window.hcaptcha && state.captchaWidgetID != null) {
      window.hcaptcha.reset(state.captchaWidgetID);
    }
  }

  function isValidAddress(v) {
    return /^0x[0-9a-fA-F]{40}$/.test(v || "");
  }

  function newIdempotencyKey() {
    if (window.crypto && typeof window.crypto.randomUUID === "function") {
      return window.crypto.randomUUID();
    }
    var rand = Math.random().toString(36).slice(2);
    return "claim-" + Date.now().toString(36) + "-" + rand;
  }

  function formatClaimState(status) {
    var s = String(status || "").toLowerCase();
    if (s === "queued") return ["Queued for processing.", "info"];
    if (s === "sending") return ["Transaction is being sent.", "info"];
    if (s === "sent") return ["Transaction sent; waiting for confirmations.", "info"];
    if (s === "confirmed") return ["Claim confirmed on-chain.", "success"];
    if (s === "failed") return ["Claim failed.", "error"];
    if (s === "rejected") return ["Claim rejected.", "warn"];
    return ["Claim status: " + (status || "unknown"), "info"];
  }

  function esc(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  async function readJSON(response) {
    try {
      return await response.json();
    } catch (_) {
      return {};
    }
  }


  function formatDecimalAmount(amountWei, decimals) {
    var raw = String(amountWei || "").trim();
    if (!/^\d+$/.test(raw)) {
      return "";
    }
    var places = Number(decimals || 0);
    if (!Number.isFinite(places) || places < 0) {
      places = 0;
    }
    if (places === 0) {
      return raw;
    }
    while (raw.length <= places) {
      raw = "0" + raw;
    }
    var whole = raw.slice(0, raw.length - places) || "0";
    var frac = raw.slice(raw.length - places).replace(/0+$/, "");
    return frac ? whole + "." + frac : whole;
  }

  function formatTokenAmount(token) {
    if (!token || !token.amount_wei) {
      return "";
    }
    var display = formatDecimalAmount(token.amount_wei, token.decimals);
    if (!display) {
      display = token.amount_wei + " base units";
    }
    return token.symbol ? display + " " + token.symbol : display;
  }

  function tokenOptionLabel(token) {
    var label = token.symbol || token.id || "Token";
    if (token.id && token.id !== label) {
      label += " (" + token.id + ")";
    }
    if (token.type) {
      label += " · " + token.type;
    }
    return label;
  }

  function selectedTokenID() {
    var select = el("token-id");
    if (!select || select.disabled || !select.value) {
      return "";
    }
    return select.value;
  }

  function selectedToken() {
    var id = selectedTokenID();
    for (var i = 0; i < state.tokens.length; i += 1) {
      if (state.tokens[i].id === id) {
        return state.tokens[i];
      }
    }
    return null;
  }

  function updateTokenStatus(text, kind) {
    var status = el("token-status");
    if (!status) {
      return;
    }
    status.textContent = text || "";
    status.className = "token-status " + (kind || "info") + (text ? "" : " hidden");
  }

  function renderTokenDetails(token) {
    var details = el("token-details");
    if (!details) {
      return;
    }
    if (!token) {
      details.innerHTML = "";
      setHidden(details, true);
      return;
    }
    var rows = [
      ["Amount", formatTokenAmount(token) || "Configured"],
      ["Type", token.type ? String(token.type).toUpperCase() : "Token"],
      ["Decimals", token.decimals != null ? String(token.decimals) : "n/a"],
    ];
    details.innerHTML = rows.map(function (r) {
      return '<div class="token-pill"><span>' + esc(r[0]) + '</span><strong>' + esc(r[1]) + '</strong></div>';
    }).join("");
    setHidden(details, false);
  }

  function updateTokenNote() {
    var note = el("token-note");
    var token = selectedToken();
    if (!note) {
      return;
    }
    if (!token) {
      note.textContent = "Default faucet token will be requested.";
      setHidden(note, true);
      renderTokenDetails(null);
      return;
    }
    var parts = [];
    parts.push("Selected " + (token.symbol || token.id));
    if (token.id) {
      parts.push("id " + token.id);
    }
    note.textContent = parts.join(" · ");
    setHidden(note, false);
    renderTokenDetails(token);
  }

  function renderTokens(tokens) {
    var row = el("token-row");
    var select = el("token-id");
    if (!row || !select) {
      return;
    }

    state.tokens = Array.isArray(tokens) ? tokens.filter(function (token) {
      return token && token.id;
    }) : [];

    select.innerHTML = "";
    if (state.tokens.length === 0) {
      state.selectedTokenID = "";
      select.disabled = true;
      setHidden(row, true);
      updateTokenStatus("", "info");
      updateTokenNote();
      return;
    }

    state.tokens.forEach(function (token) {
      var option = document.createElement("option");
      option.value = token.id;
      option.textContent = tokenOptionLabel(token);
      select.appendChild(option);
    });

    if (state.selectedTokenID) {
      select.value = state.selectedTokenID;
    }
    if (!select.value && state.tokens[0]) {
      select.value = state.tokens[0].id;
    }
    state.selectedTokenID = select.value || "";
    select.disabled = false;
    setHidden(row, false);
    updateTokenStatus("Token catalog loaded. Select the asset to request.", "info");
    updateTokenNote();
  }

  function renderTokenFallback(message) {
    var row = el("token-row");
    var select = el("token-id");
    state.tokens = [];
    state.selectedTokenID = "";
    if (select) {
      select.innerHTML = "";
      select.disabled = true;
    }
    setHidden(row, false);
    updateTokenStatus(message || "Token catalog unavailable. Default faucet token will be requested.", "warn");
    updateTokenNote();
  }

  async function loadTokens() {
    var row = el("token-row");
    var select = el("token-id");
    if (select) {
      select.disabled = true;
    }
    setHidden(row, false);
    updateTokenStatus("Loading token catalog...", "info");
    var response = await fetch("/api/v1/tokens", { headers: { Accept: "application/json" } });
    var data = await readJSON(response);
    if (!response.ok) {
      throw new Error("token catalog unavailable");
    }
    renderTokens(data.tokens || []);
  }

  async function loadConfig() {
    var response = await fetch("/api/v1/config", { headers: { Accept: "application/json" } });
    var data = await readJSON(response);

    state.explorerTxURL = data.explorer_tx_url || "";
    state.explorerURLSafe = validExplorerTemplate(state.explorerTxURL);

    var banner = el("network-banner");
    if (data.network_name) {
      banner.textContent = "Network: " + data.network_name + (data.symbol ? " | " + data.symbol : "");
      banner.className = "network-banner visible";
    } else {
      banner.className = "network-banner";
    }

    var rows = [];
    if (data.network_name) rows.push(["Network", data.network_name]);
    if (data.symbol) rows.push(["Symbol", data.symbol]);
    if (data.amount_wei) rows.push(["Amount", data.amount_wei + (data.symbol ? " base units of " + data.symbol : " base units")]);
    if (data.cooldown_seconds != null) rows.push(["Cooldown", String(data.cooldown_seconds) + "s"]);
    if (data.dry_run != null) rows.push(["Dry Run", data.dry_run ? "yes" : "no"]);

    var configCard = el("config-card");
    var configKV = el("config-kv");
    if (rows.length > 0) {
      configKV.innerHTML = rows
        .map(function (r) {
          return '<div class="kv"><span class="key">' + esc(r[0]) + '</span><span class="val">' + esc(r[1]) + "</span></div>";
        })
        .join("");
      setHidden(configCard, false);
    } else {
      setHidden(configCard, true);
    }

    await setupCaptcha(data.captcha_provider || "disabled", data.captcha_site_key || "");
  }

  async function loadPublicStatus() {
    var response = await fetch("/api/v1/status", { headers: { Accept: "application/json" } });
    var data = await readJSON(response);
    var mode = String(data.status || data.mode || "active").toLowerCase();

    var msg = "";
    var kind = "warn";
    var disabled = false;

    if (mode === "paused") {
      msg = "Faucet is currently paused.";
      disabled = true;
    } else if (mode === "maintenance") {
      msg = "Faucet is under maintenance.";
      disabled = true;
    } else if (mode === "no_funds") {
      msg = "Faucet currently has no funds.";
      disabled = true;
    }

    setStatus(el("pause-banner"), msg, kind);
    el("btn-send").disabled = disabled;
  }


  function claimTokenLabel(data) {
    if (!data) {
      return "Default token";
    }
    var symbol = data.token_symbol || data.symbol || "";
    var id = data.token_id || "";
    if (symbol && id && symbol !== id) {
      return symbol + " (" + id + ")";
    }
    return symbol || id || "Default token";
  }

  function claimAmountDisplay(data) {
    if (!data || !data.amount_wei) {
      return "";
    }
    var decimals = data.token_decimals;
    var symbol = data.token_symbol || data.symbol || "";
    var display = formatDecimalAmount(data.amount_wei, decimals);
    if (!display) {
      return data.amount_wei + " base units";
    }
    if (symbol) {
      display += " " + symbol;
    }
    return display + " (" + data.amount_wei + " base units)";
  }

  function renderClaimSummary(data) {
    var summary = el("claim-summary");
    if (!summary || !data) {
      return;
    }
    var status = String(data.status || "unknown");
    var tokenLabel = claimTokenLabel(data);
    var amount = claimAmountDisplay(data) || "Configured faucet amount";
    var type = data.token_type ? String(data.token_type).toUpperCase() : "DEFAULT";
    summary.innerHTML =
      '<div class="claim-summary-main">' +
        '<span class="claim-badge">' + esc(status) + '</span>' +
        '<strong>' + esc(amount) + '</strong>' +
        '<span>' + esc(tokenLabel + " · " + type) + '</span>' +
      '</div>';
    setHidden(summary, false);
  }

  function renderRows(target, rows) {
    if (!target) {
      return;
    }
    target.innerHTML = rows
      .map(function (r) {
        return '<div class="kv"><span class="key">' + esc(r[0]) + '</span><span class="val">' + esc(r[1]) + "</span></div>";
      })
      .join("");
  }

  function budgetLabel(budget) {
    if (!budget) {
      return "";
    }
    var remaining = budget.remaining_wei != null ? String(budget.remaining_wei) : "";
    var total = budget.budget_wei != null ? String(budget.budget_wei) : "";
    if (remaining && total) {
      return remaining + " of " + total + " wei remaining";
    }
    return remaining || total;
  }

  function renderAddressStatus(data) {
    var card = el("address-status-card");
    var summary = el("address-status-summary");
    var kv = el("address-status-kv");
    if (!card || !summary || !kv || !data) {
      return;
    }
    var eligible = !!data.eligible;
    var reason = data.reason || (eligible ? "eligible" : "not eligible");
    summary.innerHTML =
      '<strong>' + esc(eligible ? "Eligible to claim" : "Not eligible now") + '</strong>' +
      '<span>' + esc(reason) + '</span>';
    var rows = [];
    if (data.address) rows.push(["Address", data.address]);
    rows.push(["Eligible", eligible ? "yes" : "no"]);
    if (data.reason) rows.push(["Reason", data.reason]);
    if (data.cooldown_remaining_seconds != null) rows.push(["Cooldown Remaining", String(data.cooldown_remaining_seconds) + "s"]);
    if (data.next_eligible_time) rows.push(["Next Eligible", data.next_eligible_time]);
    if (data.default_token_id) rows.push(["Default Token", data.default_token_id]);
    if (data.daily_budget) rows.push(["Daily Budget", budgetLabel(data.daily_budget)]);
    if (Array.isArray(data.tokens) && data.tokens.length) {
      rows.push(["Tokens", data.tokens.map(function (token) {
        var label = token.symbol || token.token_id || "token";
        return label + ": " + (token.eligible ? "eligible" : (token.reason || "not eligible"));
      }).join(" | ")]);
    }
    renderRows(kv, rows);
    setHidden(card, false);
    card.focus && card.focus();
  }

  function renderHistory(data) {
    var card = el("address-history-card");
    var list = el("address-history-list");
    var status = el("address-history-status");
    if (!card || !list || !data) {
      return;
    }
    var claims = Array.isArray(data.claims) ? data.claims : [];
    if (claims.length === 0) {
      list.innerHTML = "";
      setStatus(status, "No public claims found for this address.", "info");
      setHidden(card, false);
      card.focus && card.focus();
      return;
    }
    setStatus(status, "Showing latest " + claims.length + " public claim" + (claims.length === 1 ? "" : "s") + ".", "info");
    list.innerHTML = claims.map(function (claim) {
      var href = txExplorerHref(claim.tx_hash);
      var tx = href ? '<a href="' + esc(href) + '" target="_blank" rel="noopener noreferrer">Explorer</a>' : esc(claim.tx_hash || "No transaction yet");
      return '<article class="history-item">' +
        '<div class="history-item-header"><strong>' + esc(claim.id || "claim") + '</strong><span class="claim-badge">' + esc(claim.status || "unknown") + '</span></div>' +
        '<div class="history-meta">Token: ' + esc(claimTokenLabel(claim)) + '</div>' +
        '<div class="history-meta">Amount: ' + esc(claimAmountDisplay(claim) || claim.amount_wei || "configured") + '</div>' +
        '<div class="history-meta">Created: ' + esc(claim.created_at || "n/a") + '</div>' +
        '<div class="history-meta">Tx: ' + tx + '</div>' +
      '</article>';
    }).join("");
    setHidden(card, false);
    card.focus && card.focus();
  }

  function renderClaim(data) {
    renderClaimSummary(data);
    var rows = [];
    if (data.id) rows.push(["Claim ID", data.id]);
    if (data.status) rows.push(["Status", data.status]);
    if (data.token_id || data.token_symbol) rows.push(["Token", claimTokenLabel(data)]);
    if (data.token_type) rows.push(["Token Type", String(data.token_type).toUpperCase()]);
    if (data.address) rows.push(["Address", data.address]);
    if (data.amount_wei) rows.push(["Amount", claimAmountDisplay(data) || data.amount_wei + " base units"]);
    if (data.tx_hash) rows.push(["Tx Hash", data.tx_hash]);
    if (data.created_at) rows.push(["Created", data.created_at]);
    if (data.updated_at) rows.push(["Updated", data.updated_at]);

    renderRows(el("claim-kv"), rows);
    setHidden(el("claim-result"), false);

    var link = el("explorer-link");
    var href = txExplorerHref(data.tx_hash);
    if (href) {
      link.className = "status visible info";
      link.innerHTML = '<a href="' + esc(href) + '" target="_blank" rel="noopener noreferrer">View transaction on explorer</a>';
      return;
    }
    link.className = "status hidden";
    link.innerHTML = "";
  }

  function renderClaimError(data) {
    var code = String((data && data.code) || "claim_unavailable");
    var details = (data && data.details) || {};

    if (code === "rate_limited") {
      var retry = details.retry_after_seconds;
      var suffix = retry != null ? " Retry in " + retry + "s." : "";
      setStatus(el("msg"), "Rate limited." + suffix, "warn");
      return;
    }
    if (code === "captcha_failed") {
      setStatus(el("msg"), "Captcha verification failed. Please refresh token and retry.", "error");
      return;
    }
    if (code === "claim_rejected") {
      if (details.reason === "invalid_token") {
        setStatus(el("msg"), "Selected token is not available. Refresh the page and retry.", "warn");
        return;
      }
      setStatus(el("msg"), "Claim rejected by risk checks.", "warn");
      return;
    }
    if (code === "faucet_unavailable") {
      setStatus(el("msg"), "Faucet is currently unavailable.", "warn");
      return;
    }
    if (code === "claim_unavailable") {
      setStatus(el("msg"), "Claim service unavailable. Please retry shortly.", "error");
      return;
    }

    var message = (data && data.message) || "Request failed.";
    setStatus(el("msg"), message + " (" + code + ")", "error");
  }

  async function pollClaim(claimID) {
    try {
      var response = await fetch("/api/v1/claim/" + encodeURIComponent(claimID), { headers: { Accept: "application/json" } });
      var data = await readJSON(response);
      if (!response.ok) {
        return;
      }

      renderClaim(data);
      var s = String(data.status || "").toLowerCase();
      var stateText = formatClaimState(s);
      setStatus(el("msg"), stateText[0], stateText[1]);

      if (state.statusStopSet[s]) {
        if (state.pollTimer) {
          clearInterval(state.pollTimer);
          state.pollTimer = null;
        }
      }
    } catch (_) {
      // Keep polling on transient errors.
    }
  }

  function startPolling(claimID) {
    if (state.pollTimer) {
      clearInterval(state.pollTimer);
    }
    state.currentClaimID = claimID;
    pollClaim(claimID);
    state.pollTimer = setInterval(function () {
      pollClaim(claimID);
    }, 3000);
  }

  async function submitClaim(ev) {
    ev.preventDefault();

    var addr = el("address").value.trim();
    if (!addr) {
      setStatus(el("msg"), "Please enter an address.", "error");
      return;
    }
    if (!isValidAddress(addr)) {
      setStatus(el("msg"), "Invalid Ethereum address format.", "error");
      return;
    }

    var body = { address: addr };
    var tokenID = selectedTokenID();
    if (tokenID) {
      body.token_id = tokenID;
    }
    var captchaToken = state.captchaToken || el("captcha-token").value.trim();
    if (state.captchaProvider && state.captchaProvider !== "disabled" && !captchaToken) {
      setStatus(el("msg"), "Please complete the captcha challenge.", "error");
      return;
    }
    if (captchaToken) {
      body.captcha_token = captchaToken;
    }

    var submitBtn = el("btn-send");
    setBusy(submitBtn, true, "Submitting...");
    setStatus(el("msg"), "Submitting claim...", "info");

    try {
      var response = await fetch("/api/v1/claim", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": newIdempotencyKey(),
        },
        body: JSON.stringify(body),
      });
      var data = await readJSON(response);
      setBusy(submitBtn, false);

      if (!response.ok) {
        refreshCaptcha();
        renderClaimError(data);
        return;
      }

      renderClaim(data);
      var currentStatus = String(data.status || "queued").toLowerCase();
      var stateText = formatClaimState(currentStatus);
      setStatus(el("msg"), stateText[0], stateText[1]);

      var claimID = data.id || data.claim_id;
      if (claimID) {
        startPolling(claimID);
      }
      refreshCaptcha();
    } catch (_) {
      setBusy(submitBtn, false);
      setStatus(el("msg"), "Network error while submitting claim.", "error");
    }
  }

  async function loadAddressStatus() {
    var addr = currentAddress();
    if (!addr || !isValidAddress(addr)) {
      setStatus(el("msg"), "Enter a valid address before checking eligibility.", "error");
      return;
    }
    var btn = el("btn-address-status");
    setBusy(btn, true, "Checking...");
    setStatus(el("msg"), "Checking address eligibility...", "info");
    try {
      var response = await fetch("/api/v1/address/" + encodeURIComponent(addr) + "/status", { headers: { Accept: "application/json" } });
      var data = await readJSON(response);
      setBusy(btn, false);
      if (!response.ok) {
        renderClaimError(data);
        return;
      }
      renderAddressStatus(data);
      setStatus(el("msg"), "Eligibility updated.", "success");
    } catch (_) {
      setBusy(btn, false);
      setStatus(el("msg"), "Unable to load address eligibility.", "error");
    }
  }

  async function loadAddressHistory() {
    var addr = currentAddress();
    if (!addr || !isValidAddress(addr)) {
      setStatus(el("msg"), "Enter a valid address before viewing history.", "error");
      return;
    }
    var btn = el("btn-address-history");
    setBusy(btn, true, "Loading...");
    setStatus(el("address-history-status"), "Loading address history...", "info");
    setHidden(el("address-history-card"), false);
    setStatus(el("msg"), "Loading address history...", "info");
    try {
      var response = await fetch("/api/v1/address/" + encodeURIComponent(addr) + "/history?limit=10&offset=0", { headers: { Accept: "application/json" } });
      var data = await readJSON(response);
      setBusy(btn, false);
      if (!response.ok) {
        renderClaimError(data);
        return;
      }
      renderHistory(data);
      setStatus(el("msg"), "Address history loaded.", "success");
    } catch (_) {
      setBusy(btn, false);
      setStatus(el("msg"), "Unable to load address history.", "error");
    }
  }

  function bindEvents() {
    el("claim-form").addEventListener("submit", submitClaim);
    el("btn-refresh").addEventListener("click", function () {
      loadPublicStatus().catch(function () {
        setStatus(el("msg"), "Unable to refresh status.", "warn");
      });
      loadTokens().catch(function () {
        renderTokenFallback("Token catalog refresh failed. Default faucet token will be requested.");
      });
      if (state.currentClaimID) {
        pollClaim(state.currentClaimID);
      }
    });
    el("btn-address-status").addEventListener("click", function () {
      loadAddressStatus();
    });
    el("btn-address-history").addEventListener("click", function () {
      loadAddressHistory();
    });
    var tokenSelect = el("token-id");
    if (tokenSelect) {
      tokenSelect.addEventListener("change", function () {
        state.selectedTokenID = tokenSelect.value || "";
        updateTokenNote();
      });
    }
  }

  async function init() {
    bindEvents();
    try {
      await loadConfig();
    } catch (_) {
      setStatus(el("msg"), "Unable to load faucet config.", "warn");
    }
    try {
      await loadTokens();
    } catch (_) {
      renderTokenFallback("Token catalog unavailable. Default faucet token will be requested.");
    }
    try {
      await loadPublicStatus();
    } catch (_) {
      setStatus(el("msg"), "Unable to load faucet status.", "warn");
    }
  }

  window.addEventListener("DOMContentLoaded", init);
})();
