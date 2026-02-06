package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// handlePluginsPage renders the installed plugins page.
// GET /plugins
func (s *Server) handlePluginsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsName, _ := s.ctrl.WorkspaceName(ctx)

	plugins, err := s.ctrl.ListInstalledPlugins(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to list installed plugins: "+err.Error())
		return
	}

	updates, _ := s.ctrl.CheckUpdates(ctx)
	updateMap := make(map[string]string, len(updates))
	for _, u := range updates {
		updateMap[u.Name] = u.LatestVersion
	}

	data := pageData{
		WorkspaceName:    wsName,
		ActiveNav:        "plugins",
		Nonce:            nonceFromCtx(ctx),
		CSRFToken:        csrfFromCtx(ctx),
		InstalledPlugins: toInstalledPluginData(plugins, updateMap),
		PluginUpdates:    len(updates),
		Breadcrumbs: []breadcrumb{
			{Label: "Installed Plugins", Href: "/plugins"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "plugins_list", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "plugins", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleInstallPlugin installs a plugin from the marketplace.
// POST /plugins/install
func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	ref := r.FormValue("ref")
	if ref == "" {
		http.Error(w, "plugin reference required", http.StatusBadRequest)
		return
	}

	result, err := s.ctrl.InstallPlugin(ctx, ref)
	if err != nil {
		w.Header().Set("HX-Trigger", triggerNotification("error", "Install failed: "+err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Installed %s v%s", result.Name, result.Version)
	w.Header().Set("HX-Trigger", triggerNotification("success", msg))
	w.WriteHeader(http.StatusOK)
}

// handleUninstallPlugin removes an installed plugin.
// POST /plugins/{name}/uninstall
func (s *Server) handleUninstallPlugin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "plugin name required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	pluginType := r.FormValue("type")
	if err := s.ctrl.UninstallPlugin(ctx, name, pluginType); err != nil {
		w.Header().Set("HX-Trigger", triggerNotification("error", "Uninstall failed: "+err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", triggerNotification("success", "Uninstalled "+name))
	w.WriteHeader(http.StatusOK)
}

// handleUpdatePlugin updates a plugin to a specified or latest version.
// POST /plugins/{name}/update
func (s *Server) handleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "plugin name required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	version := r.FormValue("version")
	result, err := s.ctrl.UpdatePlugin(ctx, name, version)
	if err != nil {
		w.Header().Set("HX-Trigger", triggerNotification("error", "Update failed: "+err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Updated %s to v%s", result.Name, result.Version)
	w.Header().Set("HX-Trigger", triggerNotification("success", msg))
	w.WriteHeader(http.StatusOK)
}

// handleUpdateAllPlugins updates all plugins with available updates.
// POST /plugins/update-all
func (s *Server) handleUpdateAllPlugins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	updates, err := s.ctrl.CheckUpdates(ctx)
	if err != nil {
		w.Header().Set("HX-Trigger", triggerNotification("error", "Failed to check updates: "+err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(updates) == 0 {
		w.Header().Set("HX-Trigger", triggerNotification("info", "All plugins are up to date"))
		w.WriteHeader(http.StatusOK)
		return
	}

	var updated int
	var lastErr error
	for _, u := range updates {
		if _, err := s.ctrl.UpdatePlugin(ctx, u.Name, u.LatestVersion); err != nil {
			lastErr = err
		} else {
			updated++
		}
	}

	notif := notificationEvent{Type: "success"}
	if lastErr != nil {
		notif.Type = "warning"
		notif.Message = fmt.Sprintf("Updated %d/%d plugins (last error: %s)", updated, len(updates), lastErr.Error())
	} else {
		notif.Message = fmt.Sprintf("Updated all %d plugins", updated)
	}

	b, _ := json.Marshal(map[string]any{"showNotification": notif})
	w.Header().Set("HX-Trigger", string(b))
	w.WriteHeader(http.StatusOK)
}

// triggerNotification builds the HX-Trigger JSON for a notification.
func triggerNotification(notifType, message string) string {
	b, _ := json.Marshal(map[string]any{
		"showNotification": notificationEvent{
			Type:    notifType,
			Message: message,
		},
	})
	return string(b)
}

// installedPluginData holds data for the installed plugins list.
type installedPluginData struct {
	Name        string
	Type        string
	Version     string
	Source      string
	InstalledAt string
	Verified    bool
	UpdateAvail string
	BuiltIn     bool
}

// toInstalledPluginData converts installed plugins to view data.
func toInstalledPluginData(plugins []ui.InstalledPlugin, updateMap map[string]string) []installedPluginData {
	result := make([]installedPluginData, len(plugins))
	for i, p := range plugins {
		update := updateMap[p.Name]
		if update == "" {
			update = p.UpdateAvail
		}
		result[i] = installedPluginData{
			Name:        p.Name,
			Type:        p.Type,
			Version:     p.Version,
			Source:      p.Source,
			InstalledAt: formatTime(p.InstalledAt),
			Verified:    p.Verified,
			UpdateAvail: update,
			BuiltIn:     p.Source == "built-in",
		}
	}
	return result
}

// formatTime returns a human-friendly date string or empty if zero.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
