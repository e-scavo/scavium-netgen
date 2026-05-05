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
      return;
    }
    node.className = "status visible " + (kind || "info");
    node.textContent = text;
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


  function formatTokenAmount(token) {
    if (!token || !token.amount_wei) {
      return "";
    }
    var suffix = token.symbol ? " base units of " + token.symbol : " base units";
    return token.amount_wei + suffix;
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

  function updateTokenNote() {
    var note = el("token-note");
    var token = selectedToken();
    if (!note) {
      return;
    }
    if (!token) {
      note.textContent = "Default faucet token will be requested.";
      setHidden(note, true);
      return;
    }
    var parts = [];
    parts.push("Selected " + (token.symbol || token.id));
    if (token.type) {
      parts.push(String(token.type).toUpperCase());
    }
    var amount = formatTokenAmount(token);
    if (amount) {
      parts.push("amount " + amount);
    }
    note.textContent = parts.join(" · ");
    setHidden(note, false);
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
    updateTokenNote();
  }

  async function loadTokens() {
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
    var mode = String(data.mode || "active").toLowerCase();

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

  function renderClaim(data) {
    var rows = [];
    if (data.id) rows.push(["Claim ID", data.id]);
    if (data.status) rows.push(["Status", data.status]);
    if (data.address) rows.push(["Address", data.address]);
    if (data.amount_wei) rows.push(["Amount", data.amount_wei + (data.symbol ? " base units of " + data.symbol : " base units")]);
    if (data.tx_hash) rows.push(["Tx Hash", data.tx_hash]);
    if (data.created_at) rows.push(["Created", data.created_at]);
    if (data.updated_at) rows.push(["Updated", data.updated_at]);

    el("claim-kv").innerHTML = rows
      .map(function (r) {
        return '<div class="kv"><span class="key">' + esc(r[0]) + '</span><span class="val">' + esc(r[1]) + "</span></div>";
      })
      .join("");
    setHidden(el("claim-result"), false);

    var link = el("explorer-link");
    if (data.tx_hash && state.explorerTxURL) {
      var href = state.explorerTxURL.replace("{txHash}", encodeURIComponent(data.tx_hash));
      link.className = "status visible info";
      link.innerHTML = '<a href="' + esc(href) + '" target="_blank" rel="noopener">View on explorer</a>';
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
    submitBtn.disabled = true;
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
      submitBtn.disabled = false;

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
      submitBtn.disabled = false;
      setStatus(el("msg"), "Network error while submitting claim.", "error");
    }
  }

  function bindEvents() {
    el("claim-form").addEventListener("submit", submitClaim);
    el("btn-refresh").addEventListener("click", function () {
      loadPublicStatus().catch(function () {
        setStatus(el("msg"), "Unable to refresh status.", "warn");
      });
      loadTokens().catch(function () {
        // Keep current/default token behavior when catalog refresh fails.
      });
      if (state.currentClaimID) {
        pollClaim(state.currentClaimID);
      }
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
      renderTokens([]);
    }
    try {
      await loadPublicStatus();
    } catch (_) {
      setStatus(el("msg"), "Unable to load faucet status.", "warn");
    }
  }

  window.addEventListener("DOMContentLoaded", init);
})();
