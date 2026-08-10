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

// This file is the desktop GUI's REMOTE mode (#60): when launched with
// --server, the app talks to a thaimaturgy-server over internal/apiclient
// instead of the in-process core. It is intentionally a lightweight client
// (library + a session view with transcript, party, command/oracle input, and a
// live SSE log); the full-fidelity experience remains local mode or the web UI.
// The in-process path is untouched.

// bg returns a short-lived context for a one-shot remote call.
func bg(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}

// showRemoteLibrary lists adventures and sessions fetched from the server.
func (g *gui) showRemoteLibrary() {
	g.stopRemoteSession()

	newBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), g.showRemoteSettings)
	hero := modernLibraryHero("thAImaturgy", "Connected to "+g.remote.BaseURL(), container.NewHBox(newBtn))

	list := container.NewVBox()
	refresh := func() { g.populateRemoteLibrary(list) }
	refresh()

	reload := widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), refresh)
	content := container.NewBorder(hero, nil, nil, nil,
		container.NewVScroll(container.NewVBox(reload, list)))
	g.win.SetContent(content)
}

func (g *gui) populateRemoteLibrary(list *fyne.Container) {
	list.Objects = nil
	ctx, cancel := bg(15)
	defer cancel()

	advs, err := g.remote.ListAdventures(ctx)
	if err != nil {
		list.Add(widget.NewLabel("⚠ " + err.Error()))
		list.Refresh()
		return
	}
	list.Add(widget.NewLabelWithStyle("Adventures", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for _, a := range advs {
		id, title := a.ID, a.Title
		play := widget.NewButton("▶  "+title, func() {
			cctx, ccancel := bg(15)
			defer ccancel()
			name, err := g.remote.NewSession(cctx, id)
			if err != nil {
				g.showErr(err)
				return
			}
			g.openRemoteSession(name)
		})
		list.Add(play)
	}

	sessions, err := g.remote.ListSessions(ctx)
	if err != nil {
		list.Add(widget.NewLabel("⚠ " + err.Error()))
		list.Refresh()
		return
	}
	if len(sessions) > 0 {
		list.Add(widget.NewLabelWithStyle("Resume session", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, s := range sessions {
			name := s.Name
			open := widget.NewButton("↻  "+name+" — "+s.AdventureTitle, func() { g.openRemoteSession(name) })
			open.Alignment = widget.ButtonAlignLeading
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				dctx, dcancel := bg(15)
				defer dcancel()
				if err := g.remote.DeleteSession(dctx, name); err != nil {
					g.showErr(err)
					return
				}
				g.populateRemoteLibrary(list)
			})
			del.Importance = widget.LowImportance
			list.Add(container.NewBorder(nil, nil, nil, del, open))
		}
	}
	list.Refresh()
}

// openRemoteSession opens a session against the server: transcript + party +
// command/oracle input + a live log tailed over SSE.
func (g *gui) openRemoteSession(name string) {
	ctx, cancel := bg(20)
	defer cancel()
	st, err := g.remote.Session(ctx, name)
	if err != nil {
		g.showErr(err)
		return
	}
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
		if m.Role == domain.RoleAssistant {
			appendTx("", m.Content)
		} else if m.Role == domain.RoleUser {
			appendTx("» ", m.Content)
		}
	}

	party := container.NewVBox()
	for i := range st.Characters {
		party.Objects = append(party.Objects, buildPCSheet(st.Characters[i])...)
	}
	if len(st.Characters) == 0 {
		party.Add(widget.NewLabelWithStyle("No party.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}

	logBox := container.NewVBox()
	logScroll := container.NewVScroll(logBox)

	input := widget.NewEntry()
	input.SetPlaceHolder("Ask the oracle, or type a /command…")
	send := func() {
		text := input.Text
		if text == "" {
			return
		}
		input.SetText("")
		appendTx("» ", text)
		go g.remoteSend(name, text, appendTx)
	}
	input.OnSubmitted = func(string) { send() }
	sendBtn := widget.NewButtonWithIcon("Send", theme.MailForwardIcon(), send)

	back := widget.NewButtonWithIcon("← Library", theme.NavigateBackIcon(), func() { g.showRemoteLibrary() })
	save := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		sctx, scancel := bg(15)
		defer scancel()
		if err := g.remote.SaveSession(sctx, name); err != nil {
			g.showErr(err)
		}
	})

	head := container.NewHBox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), layoutSpacer(), save)
	left := modernPanel("Party", "", container.NewVScroll(party))
	center := modernPanel("Transcript", "", transScroll)
	right := modernPanel("Live log", "", logScroll)
	body := container.NewHSplit(left, container.NewHSplit(center, right))
	body.SetOffset(0.28)
	bottom := container.NewBorder(nil, nil, nil, sendBtn, input)
	g.win.SetContent(container.NewBorder(head, bottom, nil, nil, body))

	// Tail the session log over SSE, marshalling updates onto the UI thread.
	g.stopRemoteSession()
	sctx, scancel := context.WithCancel(context.Background())
	g.remoteCancel = scancel
	go func() {
		_ = g.remote.StreamEvents(sctx, name, func(e domain.LogEntry) {
			ts := ""
			if !e.Timestamp.IsZero() {
				ts = e.Timestamp.Format("15:04") + " "
			}
			line := fmt.Sprintf("%s[%s] %s", ts, e.Type, e.Message)
			fyne.Do(func() {
				lbl := widget.NewLabel(line)
				lbl.Wrapping = fyne.TextWrapWord
				logBox.Add(lbl)
				logScroll.ScrollToBottom()
			})
		})
	}()
}

// remoteSend runs one command/oracle turn against the server and appends the
// result to the transcript (on the UI thread).
func (g *gui) remoteSend(name, text string, appendTx func(prefix, text string)) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if text[0] == '/' {
		res, err := g.remote.Command(ctx, name, text)
		fyne.Do(func() {
			switch {
			case err != nil:
				appendTx("⚠ ", err.Error())
			case res.Response != "":
				appendTx("", res.Response)
			case res.Message != "":
				appendTx("", res.Message)
			}
		})
		return
	}
	res, err := g.remote.Oracle(ctx, name, text)
	fyne.Do(func() {
		switch {
		case err != nil:
			appendTx("⚠ ", err.Error())
		case res.Error != "":
			appendTx("⚠ ", res.Error)
		default:
			appendTx("", res.Answer)
		}
	})
}

// showRemoteSettings edits the server-side config over the API.
func (g *gui) showRemoteSettings() {
	ctx, cancel := bg(15)
	defer cancel()
	cfg, err := g.remote.Config(ctx)
	if err != nil {
		g.showErr(err)
		return
	}
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
		sctx, scancel := bg(15)
		defer scancel()
		if err := g.remote.SaveConfig(sctx, cfg); err != nil {
			g.showErr(err)
			return
		}
		pop.Hide()
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

// conversationMessages returns a session's persisted conversation messages.
func conversationMessages(st *domain.SessionState) []domain.Message {
	if st == nil || st.Conversation == nil {
		return nil
	}
	return st.Conversation.Messages
}

func layoutSpacer() fyne.CanvasObject { return widget.NewLabel("") }
