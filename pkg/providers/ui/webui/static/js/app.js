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

  function markUnsaved() {
    if (!unsavedIndicator) {
      unsavedIndicator = document.getElementById("unsaved-indicator");
    }
    if (unsavedIndicator) {
      unsavedIndicator.classList.add("visible");
    }
  }

  function markSaved() {
    if (!unsavedIndicator) {
      unsavedIndicator = document.getElementById("unsaved-indicator");
    }
    if (unsavedIndicator) {
      unsavedIndicator.classList.remove("visible");
    }
  }

  document.body.addEventListener("markUnsaved", function () {
    markUnsaved();
  });
  document.body.addEventListener("markSaved", function () {
    markSaved();
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
})();
