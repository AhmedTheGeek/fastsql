package components

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/AhmedTheGeek/fastsql/app"
	"github.com/AhmedTheGeek/fastsql/drivers"
	"github.com/AhmedTheGeek/fastsql/helpers"
	"github.com/AhmedTheGeek/fastsql/internal/ssh"
	"github.com/AhmedTheGeek/fastsql/models"
)

// Field indices for the form
const (
	fieldName = iota
	fieldURL
	fieldReadOnly
	// SSH fields
	fieldSSHEnabled
	fieldSSHHost
	fieldSSHPort
	fieldSSHUser
	fieldSSHAuthMethod
	fieldSSHKeyPath
	fieldSSHPassphrase
	fieldSSHPassword
	fieldSSHRemoteHost
	fieldSSHRemotePort
)

// Auth method options
var sshAuthMethods = []string{"key", "password", "agent"}

type ConnectionForm struct {
	*tview.Flex
	*tview.Form
	StatusText   *tview.TextView
	Action       string
	sshFieldsBox *tview.Flex
}

func NewConnectionForm(connectionPages *models.ConnectionPages) *ConnectionForm {
	wrapper := tview.NewFlex()
	wrapper.SetDirection(tview.FlexColumnCSS)

	// Main form for basic connection fields
	addForm := tview.NewForm().
		SetFieldBackgroundColor(app.Styles.InverseTextColor).
		SetButtonBackgroundColor(tview.Styles.InverseTextColor).
		SetLabelColor(tview.Styles.PrimaryTextColor).
		SetFieldTextColor(tview.Styles.ContrastSecondaryTextColor)

	// Basic connection fields
	addForm.AddInputField("Name", "", 0, nil, nil)
	addForm.AddInputField("URL", "", 0, nil, nil)
	addForm.AddCheckbox("Read-Only", false, nil)

	// SSH Section header using a checkbox to toggle
	addForm.AddCheckbox("SSH Tunnel", false, nil)

	// SSH fields - initially added, visibility controlled by update function
	addForm.AddInputField("  SSH Host", "", 0, nil, nil)
	addForm.AddInputField("  SSH Port", "22", 0, nil, nil)
	addForm.AddInputField("  SSH User", "", 0, nil, nil)
	addForm.AddDropDown("  Auth Method", sshAuthMethods, 0, nil)
	addForm.AddInputField("  Key Path", defaultKeyPath(), 0, nil, nil)
	addForm.AddPasswordField("  Passphrase", "", 0, '*', nil)
	addForm.AddPasswordField("  Password", "", 0, '*', nil)
	addForm.AddInputField("  Remote Host", "localhost", 0, nil, nil)
	addForm.AddInputField("  Remote Port", "", 0, nil, nil)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)

	saveButton := tview.NewButton("[yellow]F1 [dark]Save")
	saveButton.SetStyle(tcell.StyleDefault.Background(app.Styles.PrimaryTextColor))
	saveButton.SetBorder(true)

	buttonsWrapper.AddItem(saveButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	testButton := tview.NewButton("[yellow]F2 [dark]Test")
	testButton.SetStyle(tcell.StyleDefault.Background(app.Styles.PrimaryTextColor))
	testButton.SetBorder(true)

	buttonsWrapper.AddItem(testButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[yellow]F3 [dark]Connect")
	connectButton.SetStyle(tcell.StyleDefault.Background(app.Styles.PrimaryTextColor))
	connectButton.SetBorder(true)

	buttonsWrapper.AddItem(connectButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	cancelButton := tview.NewButton("[yellow]Esc [dark]Cancel")
	cancelButton.SetStyle(tcell.StyleDefault.Background(tcell.Color(app.Styles.PrimaryTextColor)))
	cancelButton.SetBorder(true)

	buttonsWrapper.AddItem(cancelButton, 0, 1, false)

	statusText := tview.NewTextView()
	statusText.SetBorderPadding(1, 1, 0, 0)

	wrapper.AddItem(addForm, 0, 1, true)
	wrapper.AddItem(statusText, 4, 0, false)
	wrapper.AddItem(buttonsWrapper, 3, 0, false)

	form := &ConnectionForm{
		Flex:       wrapper,
		Form:       addForm,
		StatusText: statusText,
	}

	// Set up SSH enabled checkbox callback
	sshEnabledCheckbox := addForm.GetFormItem(fieldSSHEnabled).(*tview.Checkbox)
	sshEnabledCheckbox.SetChangedFunc(func(checked bool) {
		form.updateSSHFieldsVisibility()
		App.ForceDraw()
	})

	// Set up auth method dropdown callback
	authDropdown := addForm.GetFormItem(fieldSSHAuthMethod).(*tview.DropDown)
	authDropdown.SetSelectedFunc(func(text string, index int) {
		form.updateSSHFieldsVisibility()
		App.ForceDraw()
	})

	// Initial visibility update
	form.updateSSHFieldsVisibility()

	wrapper.SetInputCapture(form.inputCapture(connectionPages))

	return form
}

// defaultKeyPath returns the default SSH key path
func defaultKeyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/id_rsa"
	}
	return home + "/.ssh/id_rsa"
}

// updateSSHFieldsVisibility shows/hides SSH fields based on checkbox and auth method
func (form *ConnectionForm) updateSSHFieldsVisibility() {
	sshEnabled := form.GetFormItem(fieldSSHEnabled).(*tview.Checkbox).IsChecked()

	// Get references to all SSH fields
	sshHost := form.GetFormItem(fieldSSHHost).(*tview.InputField)
	sshPort := form.GetFormItem(fieldSSHPort).(*tview.InputField)
	sshUser := form.GetFormItem(fieldSSHUser).(*tview.InputField)
	authMethod := form.GetFormItem(fieldSSHAuthMethod).(*tview.DropDown)
	keyPath := form.GetFormItem(fieldSSHKeyPath).(*tview.InputField)
	passphrase := form.GetFormItem(fieldSSHPassphrase).(*tview.InputField)
	password := form.GetFormItem(fieldSSHPassword).(*tview.InputField)
	remoteHost := form.GetFormItem(fieldSSHRemoteHost).(*tview.InputField)
	remotePort := form.GetFormItem(fieldSSHRemotePort).(*tview.InputField)

	// Get current auth method
	_, authMethodText := authMethod.GetCurrentOption()

	if sshEnabled {
		// Show all basic SSH fields
		sshHost.SetLabel("  SSH Host")
		sshPort.SetLabel("  SSH Port")
		sshUser.SetLabel("  SSH User")
		authMethod.SetLabel("  Auth Method")
		remoteHost.SetLabel("  Remote Host")
		remotePort.SetLabel("  Remote Port")

		// Show/hide auth-specific fields based on auth method
		switch authMethodText {
		case "key":
			keyPath.SetLabel("  Key Path")
			passphrase.SetLabel("  Passphrase")
			password.SetLabel("") // Hide password
		case "password":
			keyPath.SetLabel("")     // Hide key path
			passphrase.SetLabel("")  // Hide passphrase
			password.SetLabel("  Password")
		case "agent":
			keyPath.SetLabel("")     // Hide key path
			passphrase.SetLabel("")  // Hide passphrase
			password.SetLabel("")    // Hide password
		}
	} else {
		// Hide all SSH fields by clearing labels
		sshHost.SetLabel("")
		sshPort.SetLabel("")
		sshUser.SetLabel("")
		authMethod.SetLabel("")
		keyPath.SetLabel("")
		passphrase.SetLabel("")
		password.SetLabel("")
		remoteHost.SetLabel("")
		remotePort.SetLabel("")
	}
}

func (form *ConnectionForm) inputCapture(connectionPages *models.ConnectionPages) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			connectionPages.SwitchToPage(pageNameConnectionSelection)
		} else if event.Key() == tcell.KeyF1 || event.Key() == tcell.KeyEnter {
			connectionName := form.GetFormItem(fieldName).(*tview.InputField).GetText()

			if connectionName == "" {
				form.StatusText.SetText("Connection name is required").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
				return event
			}

			connectionString := form.GetFormItem(fieldURL).(*tview.InputField).GetText()

			parsed, err := helpers.ParseConnectionString(connectionString)
			if err != nil {
				form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
				return event
			}

			databases := app.App.Connections()
			newDatabases := make([]models.Connection, len(databases))

			DBName := strings.Split(parsed.Normalize(",", "NULL", 0), ",")[3]

			if DBName == "NULL" {
				DBName = ""
			}

			readOnly := form.GetFormItem(fieldReadOnly).(*tview.Checkbox).IsChecked()

			parsedDatabaseData := models.Connection{
				Name:     connectionName,
				Provider: parsed.Driver,
				DBName:   DBName,
				URL:      connectionString,
				ReadOnly: readOnly,
				SSH:      form.getSSHConfig(),
			}

			switch form.Action {
			case actionNewConnection:

				newDatabases = append(databases, parsedDatabaseData)
				err := app.App.SaveConnections(newDatabases)
				if err != nil {
					form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
					return event
				}

			case actionEditConnection:
				newDatabases = make([]models.Connection, len(databases))
				row, _ := connectionsTable.GetSelection()

				for i, database := range databases {
					if i == row {
						newDatabases[i] = parsedDatabaseData

						// newDatabases[i].Name = connectionName
						// newDatabases[i].Provider = database.Provider
						// newDatabases[i].User = parsed.User.Username()
						// newDatabases[i].Password, _ = parsed.User.Password()
						// newDatabases[i].Host = parsed.Hostname()
						// newDatabases[i].Port = parsed.Port()
						// newDatabases[i].Query = parsed.Query().Encode()
						// newDatabases[i].DBName = helpers.ParsedDBName(parsed.Path)
						// newDatabases[i].DSN = parsed.DSN
					} else {
						newDatabases[i] = database
					}
				}

				err := app.App.SaveConnections(newDatabases)
				if err != nil {
					form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
					return event

				}
			}

			connectionsTable.SetConnections(newDatabases)
			connectionPages.SwitchToPage(pageNameConnectionSelection)

		} else if event.Key() == tcell.KeyF2 {
			connectionString := form.GetFormItem(fieldURL).(*tview.InputField).GetText()
			sshConfig := form.getSSHConfig()
			go form.testConnection(connectionString, sshConfig)
		}
		return event
	}
}

// getSSHConfig extracts SSH configuration from form fields
func (form *ConnectionForm) getSSHConfig() *models.SSHConfig {
	sshEnabled := form.GetFormItem(fieldSSHEnabled).(*tview.Checkbox).IsChecked()

	if !sshEnabled {
		return nil
	}

	sshPort := 22
	if portStr := form.GetFormItem(fieldSSHPort).(*tview.InputField).GetText(); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			sshPort = p
		}
	}

	remotePort := 0
	if portStr := form.GetFormItem(fieldSSHRemotePort).(*tview.InputField).GetText(); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			remotePort = p
		}
	}

	_, authMethod := form.GetFormItem(fieldSSHAuthMethod).(*tview.DropDown).GetCurrentOption()

	return &models.SSHConfig{
		Enabled:          true,
		Host:             form.GetFormItem(fieldSSHHost).(*tview.InputField).GetText(),
		Port:             sshPort,
		User:             form.GetFormItem(fieldSSHUser).(*tview.InputField).GetText(),
		AuthMethod:       authMethod,
		KeyPath:          form.GetFormItem(fieldSSHKeyPath).(*tview.InputField).GetText(),
		Passphrase:       form.GetFormItem(fieldSSHPassphrase).(*tview.InputField).GetText(),
		Password:         form.GetFormItem(fieldSSHPassword).(*tview.InputField).GetText(),
		TunnelRemoteHost: form.GetFormItem(fieldSSHRemoteHost).(*tview.InputField).GetText(),
		TunnelRemotePort: remotePort,
	}
}

// setSSHConfig populates form fields from SSH configuration
func (form *ConnectionForm) setSSHConfig(cfg *models.SSHConfig) {
	if cfg == nil {
		form.GetFormItem(fieldSSHEnabled).(*tview.Checkbox).SetChecked(false)
		// Reset SSH fields to defaults
		form.GetFormItem(fieldSSHHost).(*tview.InputField).SetText("")
		form.GetFormItem(fieldSSHPort).(*tview.InputField).SetText("22")
		form.GetFormItem(fieldSSHUser).(*tview.InputField).SetText("")
		form.GetFormItem(fieldSSHAuthMethod).(*tview.DropDown).SetCurrentOption(0)
		form.GetFormItem(fieldSSHKeyPath).(*tview.InputField).SetText(defaultKeyPath())
		form.GetFormItem(fieldSSHPassphrase).(*tview.InputField).SetText("")
		form.GetFormItem(fieldSSHPassword).(*tview.InputField).SetText("")
		form.GetFormItem(fieldSSHRemoteHost).(*tview.InputField).SetText("localhost")
		form.GetFormItem(fieldSSHRemotePort).(*tview.InputField).SetText("")
	} else {
		form.GetFormItem(fieldSSHEnabled).(*tview.Checkbox).SetChecked(cfg.Enabled)
		form.GetFormItem(fieldSSHHost).(*tview.InputField).SetText(cfg.Host)
		if cfg.Port > 0 {
			form.GetFormItem(fieldSSHPort).(*tview.InputField).SetText(strconv.Itoa(cfg.Port))
		} else {
			form.GetFormItem(fieldSSHPort).(*tview.InputField).SetText("22")
		}
		form.GetFormItem(fieldSSHUser).(*tview.InputField).SetText(cfg.User)

		// Set auth method dropdown
		authIndex := 0
		for i, m := range sshAuthMethods {
			if m == cfg.AuthMethod {
				authIndex = i
				break
			}
		}
		form.GetFormItem(fieldSSHAuthMethod).(*tview.DropDown).SetCurrentOption(authIndex)

		if cfg.KeyPath != "" {
			form.GetFormItem(fieldSSHKeyPath).(*tview.InputField).SetText(cfg.KeyPath)
		} else {
			form.GetFormItem(fieldSSHKeyPath).(*tview.InputField).SetText(defaultKeyPath())
		}
		form.GetFormItem(fieldSSHPassphrase).(*tview.InputField).SetText(cfg.Passphrase)
		form.GetFormItem(fieldSSHPassword).(*tview.InputField).SetText(cfg.Password)

		if cfg.TunnelRemoteHost != "" {
			form.GetFormItem(fieldSSHRemoteHost).(*tview.InputField).SetText(cfg.TunnelRemoteHost)
		} else {
			form.GetFormItem(fieldSSHRemoteHost).(*tview.InputField).SetText("localhost")
		}
		if cfg.TunnelRemotePort > 0 {
			form.GetFormItem(fieldSSHRemotePort).(*tview.InputField).SetText(strconv.Itoa(cfg.TunnelRemotePort))
		} else {
			form.GetFormItem(fieldSSHRemotePort).(*tview.InputField).SetText("")
		}
	}

	form.updateSSHFieldsVisibility()
}

func (form *ConnectionForm) testConnection(connectionString string, sshConfig *models.SSHConfig) {
	parsed, err := helpers.ParseConnectionString(connectionString)
	if err != nil {
		form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		App.ForceDraw()
		return
	}

	form.StatusText.SetText("Connecting...").SetTextColor(app.Styles.TertiaryTextColor)
	App.ForceDraw()

	var tunnel *ssh.Tunnel
	testConnectionString := connectionString

	// If SSH is enabled, set up the tunnel
	if sshConfig != nil && sshConfig.Enabled {
		form.StatusText.SetText("Establishing SSH tunnel...").SetTextColor(app.Styles.TertiaryTextColor)
		App.ForceDraw()

		// Find a free local port
		localPort, err := findFreePort()
		if err != nil {
			form.StatusText.SetText(fmt.Sprintf("Failed to find free port: %v", err)).
				SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
			App.ForceDraw()
			return
		}

		// Create SSH config with the local port
		sshCfg := ssh.ConfigFromSSHConfig(sshConfig)
		sshCfg.LocalPort = localPort

		// Create and start tunnel
		tunnel = ssh.New(sshCfg)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := tunnel.Start(ctx); err != nil {
			form.StatusText.SetText(fmt.Sprintf("SSH tunnel failed: %v", err)).
				SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
			App.ForceDraw()
			return
		}
		defer tunnel.Stop()

		form.StatusText.SetText("SSH tunnel established, testing database...").SetTextColor(app.Styles.TertiaryTextColor)
		App.ForceDraw()

		// Rewrite connection string to use local tunnel endpoint
		testConnectionString = rewriteConnectionForTunnel(connectionString, parsed.Driver, localPort)
	}

	var db drivers.Driver

	switch parsed.Driver {
	case drivers.DriverMySQL:
		db = &drivers.MySQL{}
	case drivers.DriverPostgres:
		db = &drivers.Postgres{}
	case drivers.DriverSqlite:
		db = &drivers.SQLite{}
	case drivers.DriverMSSQL:
		db = &drivers.MSSQL{}
	}

	err = db.TestConnection(testConnectionString)

	if err != nil {
		form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
	} else {
		msg := "Connection success"
		if tunnel != nil {
			msg = "Connection success (via SSH tunnel)"
		}
		form.StatusText.SetText(msg).SetTextColor(app.Styles.TertiaryTextColor)
	}
	App.ForceDraw()
}

// findFreePort finds an available local port for the SSH tunnel
func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// rewriteConnectionForTunnel modifies a connection string to use the local tunnel port
func rewriteConnectionForTunnel(connStr, driver string, localPort int) string {
	// For simplicity, we'll handle common patterns
	// This replaces the host:port portion with 127.0.0.1:localPort
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)

	switch driver {
	case drivers.DriverMySQL:
		// mysql://user:pass@host:port/db -> mysql://user:pass@127.0.0.1:localPort/db
		// Also handles tcp(host:port)
		if strings.Contains(connStr, "tcp(") {
			// DSN format: user:pass@tcp(host:port)/db
			start := strings.Index(connStr, "tcp(")
			end := strings.Index(connStr[start:], ")")
			if start >= 0 && end > 0 {
				return connStr[:start] + "tcp(" + localAddr + ")" + connStr[start+end+1:]
			}
		}
		// URL format
		return replaceHostInURL(connStr, localAddr)

	case drivers.DriverPostgres:
		// postgres://user:pass@host:port/db -> postgres://user:pass@127.0.0.1:localPort/db
		return replaceHostInURL(connStr, localAddr)

	case drivers.DriverMSSQL:
		// sqlserver://user:pass@host:port/... -> sqlserver://user:pass@127.0.0.1:localPort/...
		return replaceHostInURL(connStr, localAddr)

	default:
		return connStr
	}
}

// replaceHostInURL replaces the host:port portion of a URL-style connection string
func replaceHostInURL(connStr, newHostPort string) string {
	// Find @ symbol
	atIndex := strings.LastIndex(connStr, "@")
	if atIndex < 0 {
		return connStr
	}

	// Find the next / or ? after @
	rest := connStr[atIndex+1:]
	endIndex := strings.IndexAny(rest, "/?")

	if endIndex < 0 {
		// No path or query, replace everything after @
		return connStr[:atIndex+1] + newHostPort
	}

	return connStr[:atIndex+1] + newHostPort + rest[endIndex:]
}

func (form *ConnectionForm) SetAction(action string) {
	form.Action = action
}

func (form *ConnectionForm) SetConnectionData(conn models.Connection) {
	form.GetFormItem(fieldName).(*tview.InputField).SetText(conn.Name)
	form.GetFormItem(fieldURL).(*tview.InputField).SetText(conn.URL)
	form.GetFormItem(fieldReadOnly).(*tview.Checkbox).SetChecked(conn.ReadOnly)
	form.setSSHConfig(conn.SSH)
}
