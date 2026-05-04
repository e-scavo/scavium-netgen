(function () {
  var state = {
    explorerTxURL: "",
    pollTimer: null,
    currentClaimID: "",
    statusStopSet: { confirmed: true, failed: true, rejected: true },
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
    if (data.amount_eth) rows.push(["Amount", data.amount_eth + (data.symbol ? " " + data.symbol : "")]);
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

    var provider = String(data.captcha_provider || "").toLowerCase();
    setHidden(el("captcha-row"), !(provider && provider !== "disabled"));
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
    if (data.amount_eth) rows.push(["Amount", data.amount_eth + (data.symbol ? " " + data.symbol : "")]);
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
    var captchaToken = el("captcha-token").value.trim();
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
      if (state.currentClaimID) {
        pollClaim(state.currentClaimID);
      }
    });
  }

  async function init() {
    bindEvents();
    try {
      await loadConfig();
    } catch (_) {
      setStatus(el("msg"), "Unable to load faucet config.", "warn");
    }
    try {
      await loadPublicStatus();
    } catch (_) {
      setStatus(el("msg"), "Unable to load faucet status.", "warn");
    }
  }

  window.addEventListener("DOMContentLoaded", init);
})();
