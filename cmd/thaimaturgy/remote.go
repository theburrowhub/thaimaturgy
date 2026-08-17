package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/apiclient"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/nativeui"
)

// This file is the desktop GUI's REMOTE mode (#60): with --server, the app talks
// to a thaimaturgy-server over internal/apiclient instead of the in-process core.
// It is a lightweight client (library + a session view with transcript, party,
// command/oracle input, and a live SSE log); full-fidelity play stays in local
// mode or the web UI. The in-process path is untouched.
//
// Fyne threading: every remote HTTP call runs in a background goroutine so the UI
// never blocks; only widget mutations are marshalled back via fyne.Do.

// bg returns a short-lived context for a one-shot remote call.
func bg(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}

// showRemoteLibrary lists adventures and sessions fetched from the server.
func (g *gui) showRemoteLibrary() {
	g.stopRemoteSession()

	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), g.showRemoteSettings)
	hero := modernLibraryHero("thAImaturgy", "Connected to "+g.remote.BaseURL(), container.NewHBox(settingsBtn))

	list := container.NewVBox()
	status := widget.NewLabel("")
	var reloadBtn *widget.Button
	reloadBtn = widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), func() {
		g.reloadRemoteLibrary(list, status, reloadBtn)
	})
	content := container.NewBorder(hero, nil, nil, nil,
		container.NewVScroll(container.NewVBox(container.NewHBox(reloadBtn, status), list)))
	g.win.SetContent(content)
	g.reloadRemoteLibrary(list, status, reloadBtn)
}

// reloadRemoteLibrary fetches adventures+sessions in the background and rebuilds
// the list on the UI thread.
func (g *gui) reloadRemoteLibrary(list *fyne.Container, status *widget.Label, reload *widget.Button) {
	status.SetText("Loading…")
	reload.Disable()
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		advs, aerr := g.remote.ListAdventures(ctx)
		sess, serr := g.remote.ListSessions(ctx)
		fyne.Do(func() {
			reload.Enable()
			status.SetText("")
			list.Objects = nil
			if aerr != nil {
				list.Add(widget.NewLabel("⚠ " + aerr.Error()))
				list.Refresh()
				return
			}
			list.Add(widget.NewLabelWithStyle("Adventures", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			for _, a := range advs {
				id, title := a.ID, a.Title
				list.Add(widget.NewButton("▶  "+title, func() { g.remoteNewSession(id) }))
			}
			if serr != nil {
				list.Add(widget.NewLabel("⚠ " + serr.Error()))
			} else if len(sess) > 0 {
				list.Add(widget.NewLabelWithStyle("Resume session", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
				for _, s := range sess {
					name, title := s.Name, s.AdventureTitle
					open := widget.NewButton("↻  "+name+" — "+title, func() { g.openRemoteSession(name) })
					open.Alignment = widget.ButtonAlignLeading
					del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { g.remoteDeleteSession(name, list, status, reload) })
					del.Importance = widget.LowImportance
					list.Add(container.NewBorder(nil, nil, nil, del, open))
				}
			}
			list.Refresh()
		})
	}()
}

func (g *gui) remoteNewSession(adventureID string) {
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		name, err := g.remote.NewSession(ctx, adventureID)
		fyne.Do(func() {
			if err != nil {
				g.showErr(err)
				return
			}
			g.openRemoteSession(name)
		})
	}()
}

func (g *gui) remoteDeleteSession(name string, list *fyne.Container, status *widget.Label, reload *widget.Button) {
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		err := g.remote.DeleteSession(ctx, name)
		fyne.Do(func() {
			if err != nil {
				g.showErr(err)
				return
			}
			g.reloadRemoteLibrary(list, status, reload)
		})
	}()
}

// openRemoteSession fetches the session in the background, then builds a session
// view: transcript + party + command/oracle input + a live log tailed over SSE.
func (g *gui) openRemoteSession(name string) {
	go func() {
		ctx, cancel := bg(20)
		defer cancel()
		st, err := g.remote.Session(ctx, name)
		fyne.Do(func() {
			if err != nil {
				g.showErr(err)
				return
			}
			g.buildRemoteSession(name, st)
		})
	}()
}

func (g *gui) buildRemoteSession(name string, st *domain.SessionState) {
	g.remoteName = name

	transcript := container.NewVBox()
	transScroll := container.NewVScroll(transcript)
	appendTx := func(prefix, text string) {
		lbl := widget.NewLabel(prefix + cleanMarkdown(text))
		lbl.Wrapping = fyne.TextWrapWord
		transcript.Add(lbl)
		transScroll.ScrollToBottom()
	}
	for _, m := range conversationMessages(st) {
		switch m.Role {
		case domain.RoleAssistant:
			appendTx("", m.Content)
		case domain.RoleUser:
			appendTx("» ", m.Content)
		}
	}

	party := container.NewVBox()
	fillParty(party, st)

	logBox := container.NewVBox()
	logScroll := container.NewVScroll(logBox)

	input := widget.NewEntry()

	var sendBtn, modeBtn, beginBtn, restBtn *widget.Button
	var applyRemoteMode func(*domain.SessionState)
	busy := false
	setBusy := func(b bool) {
		busy = b
		for _, w := range []interface {
			Enable()
			Disable()
		}{input, sendBtn, modeBtn, beginBtn, restBtn} {
			if b {
				w.Disable()
			} else {
				w.Enable()
			}
		}
	}

	// runCmd sends a command or oracle input; for state-mutating commands it then
	// refetches the session and reconciles the party + mode UI.
	runCmd := func(text string, echo bool) {
		if text == "" || busy {
			return
		}
		setBusy(true)
		if echo {
			appendTx("» ", text)
		}
		go func() {
			resp, isCmd, err := g.remoteTurn(name, text)
			var fresh *domain.SessionState
			if isCmd && err == nil {
				fctx, fcancel := bg(15)
				fresh, _ = g.remote.Session(fctx, name)
				fcancel()
			}
			fyne.Do(func() {
				setBusy(false)
				if err != nil {
					appendTx("⚠ ", err.Error())
				} else if resp != "" {
					appendTx("", resp)
				}
				if fresh != nil {
					fillParty(party, fresh)
					party.Refresh()
					applyRemoteMode(fresh)
				}
			})
		}()
	}

	send := func() {
		t := input.Text
		if t == "" || busy {
			return
		}
		input.SetText("")
		runCmd(t, true)
	}
	input.OnSubmitted = func(string) { send() }
	sendBtn = widget.NewButtonWithIcon("Send", theme.MailForwardIcon(), send)

	// Oracle ↔ Virtual DM toggle + Begin/Rest, mirroring the local GUI and web:
	// they run the shared /mode, /begin and /rest commands on the server.
	modeBtn = widget.NewButton("Mode: Oracle", func() { runCmd("/mode", false) })
	beginBtn = widget.NewButtonWithIcon("Begin", theme.MediaSkipNextIcon(), func() { runCmd("/begin", false) })
	beginBtn.Importance = widget.HighImportance
	beginBtn.Hide()
	restBtn = widget.NewButtonWithIcon("Rest", theme.ViewRestoreIcon(), func() {
		go func() {
			choice := nativeui.Choice("Rest", "Short or long rest for the party?", "Short rest", "Long rest")
			if choice == 0 {
				return
			}
			k := "short"
			if choice == 2 {
				k = "long"
			}
			fyne.Do(func() { runCmd("/rest "+k, false) })
		}()
	})
	restBtn.Hide()

	applyRemoteMode = func(s *domain.SessionState) {
		dm := s != nil && s.EffectiveMode() == domain.ModeVirtualDM
		started := dm && s.GameStarted()
		if dm {
			modeBtn.SetText("Mode: Virtual DM")
			input.SetPlaceHolder("What do you do?  (Enter sends)")
		} else {
			modeBtn.SetText("Mode: Oracle")
			input.SetPlaceHolder("Ask the oracle, or type a /command…")
		}
		if dm && !started {
			beginBtn.Show()
		} else {
			beginBtn.Hide()
		}
		if dm && started {
			restBtn.Show()
		} else {
			restBtn.Hide()
		}
	}
	applyRemoteMode(st)

	back := widget.NewButtonWithIcon("← Library", theme.NavigateBackIcon(), func() { g.showRemoteLibrary() })
	save := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		go func() {
			ctx, cancel := bg(15)
			defer cancel()
			if err := g.remote.SaveSession(ctx, name); err != nil {
				fyne.Do(func() { g.showErr(err) })
			}
		}()
	})

	novelBtn := widget.NewButtonWithIcon("Novel", theme.DocumentCreateIcon(), g.openNovelEditor)
	head := container.NewHBox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layoutSpacer(), modeBtn, beginBtn, restBtn, novelBtn, save)
	left := modernPanel("Party", "", container.NewVScroll(party))
	center := modernPanel("Transcript", "", transScroll)
	right := modernPanel("Live log", "", logScroll)
	body := container.NewHSplit(left, container.NewHSplit(center, right))
	body.SetOffset(0.28)
	bottom := container.NewBorder(nil, nil, nil, sendBtn, input)
	g.win.SetContent(container.NewBorder(head, bottom, nil, nil, body))

	g.startRemoteLogStream(name, logBox, logScroll)
}

// remoteTurn runs one command/oracle turn synchronously (call from a goroutine),
// returning the text to show, whether it was a slash command, and any error.
func (g *gui) remoteTurn(name, text string) (resp string, isCmd bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if text[0] == '/' {
		res, e := g.remote.Command(ctx, name, text)
		if e != nil {
			return "", true, e
		}
		out := res.Response
		if out == "" {
			out = res.Message
		}
		// A command may ask the DM to narrate (e.g. /begin sets the opening scene
		// via ui_action "oracle"): run that oracle turn and append its narration.
		if res.UIAction == "oracle" && res.UIArg != "" {
			ans, oe := g.remote.Oracle(ctx, name, res.UIArg)
			if oe != nil {
				return out, true, oe
			}
			if ans.Error != "" {
				return out, true, fmt.Errorf("%s", ans.Error)
			}
			if ans.Answer != "" {
				if out != "" {
					out += "\n\n"
				}
				out += ans.Answer
			}
		}
		return out, true, nil
	}
	res, e := g.remote.Oracle(ctx, name, text)
	if e != nil {
		return "", false, e
	}
	if res.Error != "" {
		return "", false, fmt.Errorf("%s", res.Error)
	}
	return res.Answer, false, nil
}

// startRemoteLogStream tails the session's SSE log, reconnecting with bounded
// backoff if the stream drops, until the session is left.
func (g *gui) startRemoteLogStream(name string, logBox *fyne.Container, logScroll *container.Scroll) {
	g.stopRemoteSession()
	ctx, cancel := context.WithCancel(context.Background())
	g.remoteCancel = cancel
	appendLog := func(line string) {
		fyne.Do(func() {
			lbl := widget.NewLabel(line)
			lbl.Wrapping = fyne.TextWrapWord
			logBox.Add(lbl)
			logScroll.ScrollToBottom()
		})
	}
	go func() {
		backoff := time.Second
		for ctx.Err() == nil {
			err := g.remote.StreamEvents(ctx, name, func(e domain.LogEntry) {
				ts := ""
				if !e.Timestamp.IsZero() {
					ts = e.Timestamp.Format("15:04") + " "
				}
				appendLog(fmt.Sprintf("%s[%s] %s", ts, e.Type, e.Message))
			})
			if ctx.Err() != nil {
				return
			}
			msg := "live log disconnected; reconnecting…"
			if err != nil {
				msg = "live log error (" + err.Error() + "); reconnecting…"
			}
			appendLog("… " + msg)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 15*time.Second {
				backoff *= 2
			}
		}
	}()
}

// showRemoteSettings lets the user change the server connection (host URL + access
// token) and, when the server is reachable, edit its config (provider/model/lang).
// The connection fields default to the current values and can be changed even when
// the server is unreachable (e.g. wrong host/token), so the user can fix them here.
func (g *gui) showRemoteSettings() {
	// --- Connection: server URL + access token ---
	urlEntry := widget.NewEntry()
	urlEntry.SetText(g.remoteURL)
	urlEntry.SetPlaceHolder("http://127.0.0.1:8765")
	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetText(g.remoteToken)
	tokenEntry.SetPlaceHolder("(leave blank if the server requires no token)")
	connForm := widget.NewForm(
		widget.NewFormItem("Server URL", urlEntry),
		widget.NewFormItem("Access token", tokenEntry),
	)

	var pop *widget.PopUp
	applyConn := func() {
		url := strings.TrimSpace(urlEntry.Text)
		if url == "" {
			g.showErr(fmt.Errorf("server URL is required"))
			return
		}
		// Rebuild the client with the new host/token and reconnect. A blank token
		// connects without one (allowed when the server requires none).
		g.remoteURL = url
		g.remoteToken = strings.TrimSpace(tokenEntry.Text)
		g.remote = apiclient.New(g.remoteURL, g.remoteToken)
		g.stopRemoteSession()
		pop.Hide()
		g.showRemoteLibrary()
	}

	// --- Server settings (provider/model/language) — needs a live connection ---
	provider := widget.NewEntry()
	model := widget.NewEntry()
	lang := widget.NewEntry()
	srvForm := widget.NewForm(
		widget.NewFormItem("Provider", provider),
		widget.NewFormItem("Model", model),
		widget.NewFormItem("Language", lang),
	)
	srvForm.Hide()
	srvStatus := widget.NewLabel("Loading server settings…")
	srvStatus.Wrapping = fyne.TextWrapWord
	var loaded *domain.Config
	saveSrv := widget.NewButtonWithIcon("Save server settings", theme.DocumentSaveIcon(), func() {
		if loaded == nil {
			return
		}
		loaded.Provider = domain.ProviderType(strings.TrimSpace(provider.Text))
		loaded.Model = strings.TrimSpace(model.Text)
		loaded.Language = domain.Language(strings.TrimSpace(lang.Text))
		// Capture the client on the UI thread: "Apply & reconnect" can replace
		// g.remote concurrently, and this config belongs to THIS server — sending it
		// to a just-applied different server would be wrong (and racy).
		client := g.remote
		go func() {
			ctx, cancel := bg(15)
			defer cancel()
			err := client.SaveConfig(ctx, loaded)
			fyne.Do(func() {
				if err != nil {
					g.showErr(err)
					return
				}
				pop.Hide()
			})
		}()
	})
	saveSrv.Hide()

	content := container.NewVBox(
		widget.NewLabelWithStyle("Connection", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		connForm,
		container.NewHBox(widget.NewButtonWithIcon("Apply & reconnect", theme.ConfirmIcon(), applyConn)),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Server settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		srvStatus, srvForm, saveSrv,
		widget.NewSeparator(),
		container.NewHBox(widget.NewButton("Close", func() { pop.Hide() })),
	)
	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(480, 460))
	pop.Show()

	// Load the server config in the background; on failure the connection fields
	// above still work, so the user can correct the URL/token and reconnect.
	// Capture the client (Apply & reconnect may swap g.remote before this returns).
	client := g.remote
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		cfg, err := client.Config(ctx)
		fyne.Do(func() {
			if err != nil {
				srvStatus.SetText("Could not reach the server (" + err.Error() + "). Fix the URL/token above and Apply & reconnect.")
				return
			}
			loaded = cfg
			provider.SetText(string(cfg.Provider))
			model.SetText(cfg.Model)
			lang.SetText(string(cfg.Language))
			srvStatus.Hide()
			srvForm.Show()
			saveSrv.Show()
		})
	}()
}

// stopRemoteSession cancels the current session's SSE stream, if any.
func (g *gui) stopRemoteSession() {
	if g.remoteCancel != nil {
		g.remoteCancel()
		g.remoteCancel = nil
	}
	g.remoteName = ""
}

// fillParty (re)builds a party container from a session snapshot.
func fillParty(party *fyne.Container, st *domain.SessionState) {
	party.Objects = nil
	for i := range st.Characters {
		party.Objects = append(party.Objects, buildPCSheet(st.Characters[i])...)
	}
	if len(st.Characters) == 0 {
		party.Add(widget.NewLabelWithStyle("No party.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}
}

// conversationMessages returns a session's persisted conversation messages.
func conversationMessages(st *domain.SessionState) []domain.Message {
	if st == nil || st.Conversation == nil {
		return nil
	}
	return st.Conversation.Messages
}

func layoutSpacer() fyne.CanvasObject { return widget.NewLabel("") }
