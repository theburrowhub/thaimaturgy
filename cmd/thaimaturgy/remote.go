package main

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
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
	input.SetPlaceHolder("Ask the oracle, or type a /command…")
	var sendBtn *widget.Button
	sending := false
	send := func() {
		text := input.Text
		if text == "" || sending {
			return
		}
		sending = true
		input.SetText("")
		input.Disable()
		sendBtn.Disable()
		appendTx("» ", text)
		go func() {
			resp, isCmd, err := g.remoteTurn(name, text)
			// If a state-mutating command succeeded, refetch to refresh the party.
			var fresh *domain.SessionState
			if isCmd && err == nil {
				fctx, fcancel := bg(15)
				fresh, _ = g.remote.Session(fctx, name)
				fcancel()
			}
			fyne.Do(func() {
				sending = false
				input.Enable()
				sendBtn.Enable()
				if err != nil {
					appendTx("⚠ ", err.Error())
				} else if resp != "" {
					appendTx("", resp)
				}
				if fresh != nil {
					fillParty(party, fresh)
					party.Refresh()
				}
			})
		}()
	}
	input.OnSubmitted = func(string) { send() }
	sendBtn = widget.NewButtonWithIcon("Send", theme.MailForwardIcon(), send)

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
	head := container.NewHBox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), layoutSpacer(), novelBtn, save)
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
		if res.Response != "" {
			return res.Response, true, nil
		}
		return res.Message, true, nil
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

// showRemoteSettings edits the server-side config over the API.
func (g *gui) showRemoteSettings() {
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		cfg, err := g.remote.Config(ctx)
		fyne.Do(func() {
			if err != nil {
				g.showErr(err)
				return
			}
			g.remoteSettingsDialog(cfg)
		})
	}()
}

func (g *gui) remoteSettingsDialog(cfg *domain.Config) {
	provider := widget.NewEntry()
	provider.SetText(string(cfg.Provider))
	model := widget.NewEntry()
	model.SetText(cfg.Model)
	lang := widget.NewEntry()
	lang.SetText(string(cfg.Language))
	form := widget.NewForm(
		widget.NewFormItem("Provider", provider),
		widget.NewFormItem("Model", model),
		widget.NewFormItem("Language", lang),
	)
	var pop *widget.PopUp
	saveBtn := widget.NewButton("Save", func() {
		cfg.Provider = domain.ProviderType(provider.Text)
		cfg.Model = model.Text
		cfg.Language = domain.Language(lang.Text)
		go func() {
			ctx, cancel := bg(15)
			defer cancel()
			err := g.remote.SaveConfig(ctx, cfg)
			fyne.Do(func() {
				if err != nil {
					g.showErr(err)
					return
				}
				pop.Hide()
			})
		}()
	})
	content := container.NewVBox(
		widget.NewLabelWithStyle("Server settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form, container.NewHBox(saveBtn, widget.NewButton("Close", func() { pop.Hide() })),
	)
	pop = widget.NewModalPopUp(container.NewPadded(content), g.win.Canvas())
	pop.Resize(fyne.NewSize(420, 260))
	pop.Show()
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
