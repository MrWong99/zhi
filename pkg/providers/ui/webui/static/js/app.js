// zhi webui - client-side JavaScript
// Theme toggle, keyboard shortcuts, and notifications.
(function () {
  "use strict";

  // ---------- Theme Toggle ----------

  function getTheme() {
    var saved = localStorage.getItem("zhi-theme");
    if (saved) return saved;
    if (
      window.matchMedia &&
      window.matchMedia("(prefers-color-scheme: dark)").matches
    ) {
      return "dark";
    }
    return "light";
  }

  function setTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("zhi-theme", theme);
  }

  function toggleTheme() {
    var current =
      document.documentElement.getAttribute("data-theme") || "light";
    setTheme(current === "dark" ? "light" : "dark");
  }

  // Apply saved theme on load.
  setTheme(getTheme());

  // Bind theme toggle button (replaces inline onclick for CSP compliance).
  var themeToggleBtn = document.getElementById("theme-toggle-btn");
  if (themeToggleBtn) {
    themeToggleBtn.addEventListener("click", function (e) {
      e.preventDefault();
      toggleTheme();
    });
  }

  // ---------- Notifications ----------

  var toastContainer = null;

  function ensureToastContainer() {
    if (toastContainer) return toastContainer;
    toastContainer = document.getElementById("toast-container");
    if (!toastContainer) {
      toastContainer = document.createElement("div");
      toastContainer.id = "toast-container";
      toastContainer.className = "toast-container";
      toastContainer.setAttribute("aria-live", "polite");
      document.body.appendChild(toastContainer);
    }
    return toastContainer;
  }

  function showNotification(detail) {
    var container = ensureToastContainer();
    var toast = document.createElement("div");
    toast.className = "toast toast-" + (detail.type || "info");
    toast.setAttribute("role", "alert");

    var msg = document.createElement("span");
    msg.className = "toast-message";
    msg.textContent = detail.message || "";
    toast.appendChild(msg);

    var closeBtn = document.createElement("button");
    closeBtn.className = "toast-close";
    closeBtn.setAttribute("aria-label", "Close");
    closeBtn.innerHTML = "&times;";
    closeBtn.addEventListener("click", function () {
      toast.classList.add("toast-exit");
      setTimeout(function () {
        toast.remove();
      }, 300);
    });
    toast.appendChild(closeBtn);

    container.appendChild(toast);

    // Auto-dismiss after 5 seconds.
    setTimeout(function () {
      if (toast.parentNode) {
        toast.classList.add("toast-exit");
        setTimeout(function () {
          toast.remove();
        }, 300);
      }
    }, 5000);
  }

  // Listen for HTMX-triggered notification events.
  document.body.addEventListener("showNotification", function (evt) {
    showNotification(evt.detail);
  });

  // ---------- Unsaved Changes Indicator ----------

  var unsavedIndicator = null;
  var hasUnsaved = false;

  function markUnsaved() {
    hasUnsaved = true;
    if (!unsavedIndicator) {
      unsavedIndicator = document.getElementById("unsaved-indicator");
    }
    if (unsavedIndicator) {
      unsavedIndicator.classList.add("visible");
    }
    var saveBtn = document.getElementById("save-tree-btn");
    if (saveBtn) {
      saveBtn.disabled = false;
    }
  }

  function markSaved() {
    hasUnsaved = false;
    if (!unsavedIndicator) {
      unsavedIndicator = document.getElementById("unsaved-indicator");
    }
    if (unsavedIndicator) {
      unsavedIndicator.classList.remove("visible");
    }
    var saveBtn = document.getElementById("save-tree-btn");
    if (saveBtn) {
      saveBtn.disabled = true;
    }
  }

  document.body.addEventListener("markUnsaved", function () {
    markUnsaved();
  });
  document.body.addEventListener("markSaved", function () {
    markSaved();
  });

  // Warn before navigating away with unsaved changes.
  window.addEventListener("beforeunload", function (e) {
    if (hasUnsaved) {
      e.preventDefault();
    }
  });

  // ---------- Keyboard Shortcuts ----------

  var shortcuts = [];
  var sequence = "";
  var sequenceTimer = null;

  function registerShortcut(keys, description, handler) {
    shortcuts.push({ keys: keys, description: description, handler: handler });
  }

  function isInputFocused() {
    var el = document.activeElement;
    if (!el) return false;
    var tag = el.tagName.toLowerCase();
    return (
      tag === "input" ||
      tag === "textarea" ||
      tag === "select" ||
      el.isContentEditable
    );
  }

  document.addEventListener("keydown", function (e) {
    // Allow Ctrl+S and Esc even when input is focused.
    if (e.ctrlKey && e.key === "s") {
      e.preventDefault();
      triggerSave();
      return;
    }
    if (e.key === "Escape") {
      // Close any open edit forms.
      var cancelBtn = document.querySelector(".editor-form .btn-outline");
      if (cancelBtn) {
        cancelBtn.click();
      }
      // Blur focused input.
      if (document.activeElement) {
        document.activeElement.blur();
      }
      return;
    }

    if (isInputFocused()) return;

    // Single-key shortcuts.
    if (e.key === "/" && !e.ctrlKey && !e.metaKey) {
      e.preventDefault();
      var filterInput = document.querySelector(
        '.tree-filter input[type="search"]',
      );
      if (filterInput) {
        filterInput.focus();
      }
      return;
    }

    if (e.key === "?") {
      e.preventDefault();
      window.location.href = "/shortcuts";
      return;
    }

    // Multi-key sequences (e.g., g+t, g+c, g+v).
    if (sequenceTimer) {
      clearTimeout(sequenceTimer);
      sequenceTimer = null;
    }

    sequence += e.key;

    if (sequence === "gt") {
      sequence = "";
      window.location.href = "/tree";
      return;
    }
    if (sequence === "gc") {
      sequence = "";
      window.location.href = "/components";
      return;
    }
    if (sequence === "gv") {
      sequence = "";
      window.location.href = "/validation";
      return;
    }
    if (sequence === "ge") {
      sequence = "";
      window.location.href = "/export";
      return;
    }
    if (sequence === "ga") {
      sequence = "";
      window.location.href = "/apply";
      return;
    }
    if (sequence === "gm") {
      sequence = "";
      window.location.href = "/marketplace";
      return;
    }
    if (sequence === "gp") {
      sequence = "";
      window.location.href = "/plugins";
      return;
    }

    // Reset sequence after timeout.
    if (sequence.length > 0) {
      sequenceTimer = setTimeout(function () {
        sequence = "";
      }, 500);
    }
    if (sequence.length > 2) {
      sequence = "";
    }
  });

  function triggerSave() {
    var saveBtn = document.getElementById("save-tree-btn");
    if (saveBtn) {
      saveBtn.click();
    }
  }

  // Register shortcuts for documentation.
  registerShortcut("/", "Focus filter input", function () {});
  registerShortcut("g t", "Navigate to Tree", function () {});
  registerShortcut("g c", "Navigate to Components", function () {});
  registerShortcut("g v", "Navigate to Validation", function () {});
  registerShortcut("g e", "Navigate to Export", function () {});
  registerShortcut("g a", "Navigate to Apply", function () {});
  registerShortcut("g m", "Navigate to Marketplace", function () {});
  registerShortcut("g p", "Navigate to Plugins", function () {});
  registerShortcut("Ctrl+S", "Save tree", function () {});
  registerShortcut("Esc", "Close edit form / Cancel", function () {});
  registerShortcut("?", "Show shortcut overlay", function () {});

  // ---------- Boolean Toggle Text Sync ----------
  // When a boolean toggle checkbox is changed via HTMX-swapped content,
  // update the adjacent text label to match.
  document.body.addEventListener("change", function (e) {
    if (
      e.target &&
      e.target.classList.contains("toggle-input") &&
      e.target.type === "checkbox"
    ) {
      var textSpan = e.target.parentElement.querySelector(".toggle-text");
      if (textSpan) {
        textSpan.textContent = e.target.checked ? "true" : "false";
      }
    }
  });

  // ---------- Map & List Editor ----------

  // Remove row when clicking a map-remove-btn or list-remove-btn.
  document.body.addEventListener("click", function (e) {
    var removeBtn = e.target.closest(".map-remove-btn, .list-remove-btn");
    if (removeBtn) {
      var row = removeBtn.closest(".map-row, .list-row");
      if (row) row.remove();
      return;
    }

    // Add a new map key-value row.
    var mapAddBtn = e.target.closest(".map-add-btn");
    if (mapAddBtn) {
      var keyPh = mapAddBtn.getAttribute("data-key-placeholder") || "";
      var valPh = mapAddBtn.getAttribute("data-value-placeholder") || "";
      var row = document.createElement("div");
      row.className = "map-row";

      var keyInput = document.createElement("input");
      keyInput.type = "text";
      keyInput.name = "map_key";
      keyInput.className = "input map-key-input";
      if (keyPh) keyInput.placeholder = keyPh;

      var valInput = document.createElement("input");
      valInput.type = "text";
      valInput.name = "map_value";
      valInput.className = "input map-value-input";
      if (valPh) valInput.placeholder = valPh;

      var rmBtn = document.createElement("button");
      rmBtn.type = "button";
      rmBtn.className = "btn btn-outline btn-sm map-remove-btn";
      rmBtn.innerHTML = "\u2212";

      row.appendChild(keyInput);
      row.appendChild(valInput);
      row.appendChild(rmBtn);
      mapAddBtn.parentNode.insertBefore(row, mapAddBtn);
      keyInput.focus();
      return;
    }

    // Add a new list item row.
    var listAddBtn = e.target.closest(".list-add-btn");
    if (listAddBtn) {
      var itemPh = listAddBtn.getAttribute("data-item-placeholder") || "";
      var row = document.createElement("div");
      row.className = "list-row";

      var itemInput = document.createElement("input");
      itemInput.type = "text";
      itemInput.name = "list_item";
      itemInput.className = "input list-item-input";
      if (itemPh) itemInput.placeholder = itemPh;

      var rmBtn = document.createElement("button");
      rmBtn.type = "button";
      rmBtn.className = "btn btn-outline btn-sm list-remove-btn";
      rmBtn.innerHTML = "\u2212";

      row.appendChild(itemInput);
      row.appendChild(rmBtn);
      listAddBtn.parentNode.insertBefore(row, listAddBtn);
      itemInput.focus();
      return;
    }
  });

  // ---------- Login Page Auth Method Selector ----------

  var authMethodSelect = document.getElementById("auth-method-select");
  if (authMethodSelect) {
    authMethodSelect.addEventListener("change", function () {
      window.location.href = "/login?method=" + encodeURIComponent(this.value);
    });
  }
})();
