package desktop

import (
	"crypto/rand"
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
)

// The org.freedesktop.portal.FileChooser D-Bus API, part of the XDG desktop portal.
const (
	portalDestination = "org.freedesktop.portal.Desktop"
	portalPath        = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	openFileMethod    = "org.freedesktop.portal.FileChooser.OpenFile"
	requestInterface  = "org.freedesktop.portal.Request"
	responseMember    = "Response"
	responseSignal    = requestInterface + "." + responseMember
	requestPathPrefix = "/org/freedesktop/portal/desktop/request/"
	globRule          = 0 // the rule kind for a glob pattern such as "*.age"
	responseSuccess   = 0
	responseCancelled = 1
)

// signalBufferSize is how many signals can wait while the portal flow reads them. A full buffer only delays the bus.
const signalBufferSize = 4

var (
	errNoPortal          = errors.New("no desktop portal")
	errPortalFailed      = errors.New("the file dialog failed")
	errPortalUnreachable = errors.New("lost the connection to the file dialog")
)

// portalFilter and portalRule are the D-Bus structs (sa(us)) that the portal's "filters" option takes.
type portalFilter struct {
	Name  string
	Rules []portalRule
}

type portalRule struct {
	Kind    uint32
	Pattern string
}

// askPortal shows the portal's file chooser and waits for the answer. It returns errNoPortal if the portal isn't
// there to ask.
func askPortal(connection busConnection, dialog Dialog, kind selection) ([]string, error) {
	names := connection.Names()
	if len(names) == 0 {
		return nil, errNoPortal
	}
	// Subscribe to replies before asking, so the reply can't be missed. The match leaves out the request path: old
	// portals ignore handle_token and pick their own, which is known only once OpenFile returns.
	err := connection.AddMatchSignal(dbus.WithMatchInterface(requestInterface), dbus.WithMatchMember(responseMember))
	if err != nil {
		return nil, errNoPortal
	}
	signals := make(chan *dbus.Signal, signalBufferSize)
	connection.Signal(signals)

	token := "vita_decrypt_" + rand.Text()
	var handle dbus.ObjectPath
	err = connection.Object(portalDestination, portalPath).
		Call(openFileMethod, 0, "", dialog.Title, portalOptions(dialog, kind, token)).Store(&handle)
	if err != nil {
		return nil, errNoPortal
	}
	return awaitResponse(signals, handle, requestPath(names[0], token))
}

// portalOptions is the options argument of FileChooser.OpenFile.
func portalOptions(dialog Dialog, kind selection, token string) map[string]dbus.Variant {
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"modal":        dbus.MakeVariant(true),
		"multiple":     dbus.MakeVariant(kind == multipleFiles),
		"directory":    dbus.MakeVariant(kind == singleFolder),
	}
	if filters := portalFilters(dialog.Filters); len(filters) > 0 {
		options["filters"] = dbus.MakeVariant(filters)
	}
	if dialog.Folder != "" {
		// The portal takes the folder as a NUL-terminated byte string.
		options["current_folder"] = dbus.MakeVariant(append([]byte(dialog.Folder), 0))
	}
	return options
}

func portalFilters(filters []Filter) []portalFilter {
	var converted []portalFilter
	for _, filter := range withAllFiles(filters, "*") {
		rules := make([]portalRule, 0, len(filter.Patterns))
		for _, pattern := range filter.Patterns {
			rules = append(rules, portalRule{Kind: globRule, Pattern: pattern})
		}
		converted = append(converted, portalFilter{Name: filter.Name, Rules: rules})
	}
	return converted
}

// requestPath is where the portal puts the request object for token, when it honors handle_token.
func requestPath(uniqueName, token string) dbus.ObjectPath {
	sender := strings.ReplaceAll(strings.TrimPrefix(uniqueName, ":"), ".", "_")
	return dbus.ObjectPath(requestPathPrefix + sender + "/" + token)
}

// awaitResponse waits for the Response signal from the request at handle or request, and decodes it.
func awaitResponse(signals <-chan *dbus.Signal, handle, request dbus.ObjectPath) ([]string, error) {
	for signal := range signals {
		if isResponse(signal, handle, request) {
			return decodeResponse(signal.Body)
		}
	}
	return nil, errPortalUnreachable
}

func isResponse(signal *dbus.Signal, handle, request dbus.ObjectPath) bool {
	return (signal.Path == handle || signal.Path == request) && signal.Name == responseSignal && len(signal.Body) >= 2
}

// decodeResponse reads the body of a Response signal: a response code, then a dictionary with the chosen "uris".
func decodeResponse(body []any) ([]string, error) {
	code, _ := body[0].(uint32) // a portal that sends no code has nothing else to go on; read it as success
	switch code {
	case responseSuccess:
	case responseCancelled:
		return nil, nil
	default:
		return nil, errPortalFailed
	}
	results, _ := body[1].(map[string]dbus.Variant) // a missing dictionary means nothing was chosen
	var uris []string
	if variant, ok := results["uris"]; ok {
		_ = variant.Store(&uris) // a "uris" that isn't a list of strings means nothing was chosen
	}
	return pathsFromURIs(uris), nil
}

// pathsFromURIs turns the local file:// URIs in uris into paths, skipping any other URI.
func pathsFromURIs(uris []string) []string {
	var paths []string
	for _, uri := range uris {
		if path, ok := pathFromURI(uri); ok {
			paths = append(paths, path)
		}
	}
	return paths
}
