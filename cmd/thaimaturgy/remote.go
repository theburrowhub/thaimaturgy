package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/theburrowhub/thaimaturgy/internal/apiclient"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/engine"
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

	newBtn := widget.NewButtonWithIcon("New adventure…", theme.DocumentCreateIcon(), g.newRemoteAdventure)
	importBtn := widget.NewButtonWithIcon("Import (.tar.gz)…", theme.FolderOpenIcon(), g.remoteImportDialog)
	charsBtn := widget.NewButtonWithIcon("Characters…", theme.AccountIcon(), func() { g.showRosterManager(g.remoteRosterOps()) })
	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), g.showRemoteSettings)
	hero := modernLibraryHero("thAImaturgy", "Connected to "+g.remote.BaseURL(), container.NewHBox(g.startModeSelector(), newBtn, importBtn, charsBtn, settingsBtn))

	list := container.NewVBox()
	status := widget.NewLabel("")
	var reloadBtn *widget.Button
	reloadBtn = widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), func() {
		g.reloadRemoteLibrary(list, status, reloadBtn)
	})
	content := container.NewBorder(hero, nil, nil, nil,
		container.NewVScroll(container.NewVBox(container.NewHBox(reloadBtn, status), list)))
	g.win.SetContent(appShell(content))
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
				play := widget.NewButton("▶  "+title, func() { g.remoteNewSession(id) })
				edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { g.editRemoteAdventure(id) })
				edit.Importance = widget.LowImportance
				list.Add(container.NewBorder(nil, nil, nil, edit, play))
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

// remoteImportDialog picks a .tar.gz module and uploads it to the server, then
// refreshes the library (mirrors the local importDialog, but over the API).
func (g *gui) remoteImportDialog() {
	go func() {
		path, ok := nativeui.OpenFile("Import adventure module",
			nativeui.Filter{Name: "Adventure module", Patterns: []string{"*.tar.gz", "*.tgz", "*.gz"}})
		if !ok {
			return
		}
		ctx, cancel := bg(120) // an upload + server-side extract can take a while
		_, title, err := g.remote.ImportAdventure(ctx, path)
		cancel()
		if err != nil {
			g.showErr(err)
			return
		}
		nativeui.Info("Imported", "Imported: "+title)
		fyne.Do(func() { g.showRemoteLibrary() })
	}()
}

func (g *gui) remoteNewSession(adventureID string) {
	mode := g.startMode // capture on the UI thread; the goroutine must not read the
	// selector-mutated field concurrently (data race), and the session should use
	// the mode chosen when it was launched, not a later selector change.
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		name, err := g.remote.NewSession(ctx, adventureID)
		if err != nil {
			fyne.Do(func() { g.showErr(err) })
			return
		}
		// The session now exists on the server. Honor the mode chosen on the library
		// (a new session defaults to Oracle server-side, so only a Virtual-DM choice
		// needs applying), using its OWN context. If that fails we still OPEN the
		// created session — reporting the mode-update failure — so a retry can't
		// create a duplicate session and the created one isn't orphaned.
		var modeErr error
		if mode == domain.ModeVirtualDM {
			mctx, mcancel := bg(15)
			_, modeErr = g.remote.Command(mctx, name, "/mode dm")
			mcancel()
		}
		fyne.Do(func() {
			if modeErr != nil {
				g.showErr(fmt.Errorf("session started, but switching to Virtual DM failed (you can toggle mode inside): %w", modeErr))
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

	var refreshParty func() // set when the party panel is built (below)

	logBox := container.NewVBox()
	logScroll := container.NewVScroll(logBox)

	input := widget.NewEntry()

	var sendBtn, modeBtn, beginBtn, restBtn, tgBtn *widget.Button
	var applyRemoteMode func(*domain.SessionState)
	busy := false
	hosting := false
	curState := st // latest known session state, for placeholder/mode recompute
	refreshControls := func() {
		turnLocked := busy || hosting
		for _, w := range []interface {
			Enable()
			Disable()
		}{input, sendBtn, modeBtn, beginBtn, restBtn} {
			if turnLocked {
				w.Disable()
			} else {
				w.Enable()
			}
		}
		// The host toggle stays usable while hosting (to stop it) but not mid-turn.
		if busy {
			tgBtn.Disable()
		} else {
			tgBtn.Enable()
		}
	}
	setBusy := func(b bool) { busy = b; refreshControls() }

	// runCmd sends a command or oracle input; for state-mutating commands it then
	// refetches the session and reconciles the party + mode UI.
	runCmd := func(text string, echo bool) {
		if text == "" || busy || hosting {
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
					curState = fresh
					if refreshParty != nil {
						refreshParty()
					}
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

	// Host-on-Telegram toggle (virtual-DM only): the SERVER runs the bot bound to
	// this session, using the server-configured token. While hosting, the bot is
	// the sole driver, so this client's turn controls are disabled (the server
	// would reject them with a "hosted" error anyway).
	var setHosting func(bool)
	tgBtn = widget.NewButtonWithIcon("Host: Telegram", theme.ComputerIcon(), func() {
		if busy {
			return
		}
		wantStop := hosting
		setBusy(true)
		go func() {
			ctx, cancel := bg(30)
			var st apiclient.TelegramStatus
			var err error
			if wantStop {
				st, err = g.remote.StopTelegramHost(ctx, name)
			} else {
				st, err = g.remote.StartTelegramHost(ctx, name)
			}
			cancel()
			fyne.Do(func() {
				setBusy(false)
				if err != nil {
					appendTx("⚠ ", err.Error())
					return
				}
				setHosting(st.Hosting)
				if st.Hosting {
					msg := "Hosting on Telegram"
					if st.Username != "" {
						msg += " as @" + st.Username
					}
					appendTx("", msg+". Players drive the game from Telegram; turns here are paused until you stop hosting.")
				} else {
					appendTx("", "Stopped hosting on Telegram.")
				}
			})
		}()
	})
	tgBtn.Hide()
	setHosting = func(h bool) {
		hosting = h
		if h {
			tgBtn.SetText("Hosting — stop")
			tgBtn.Importance = widget.HighImportance
		} else {
			tgBtn.SetText("Host: Telegram")
			tgBtn.Importance = widget.MediumImportance
		}
		tgBtn.Refresh()
		refreshControls()
		// Recompute placeholder + mode readout from the latest known session so the
		// hosting placeholder is applied on start AND cleared on stop.
		applyRemoteMode(curState)
	}

	applyRemoteMode = func(s *domain.SessionState) {
		dm := s != nil && s.EffectiveMode() == domain.ModeVirtualDM
		started := dm && s.GameStarted()
		switch {
		case hosting:
			input.SetPlaceHolder("Hosting on Telegram — players drive the game there")
		case dm:
			input.SetPlaceHolder("What do you do?  (Enter sends)")
		default:
			input.SetPlaceHolder("Ask the oracle, or type a /command…")
		}
		if dm {
			modeBtn.SetText("Mode: Virtual DM")
		} else {
			modeBtn.SetText("Mode: Oracle")
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
		// The host toggle belongs to virtual-DM mode; keep it visible while
		// hosting even if the mode readout lags.
		if dm || hosting {
			tgBtn.Show()
		} else {
			tgBtn.Hide()
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
		layoutSpacer(), modeBtn, beginBtn, restBtn, tgBtn, novelBtn, save)
	partyPanel, rp := g.remotePartyPanel(name, st)
	refreshParty = rp
	left := modernPanel("Party", "", partyPanel)
	center := modernPanel("Transcript", "", transScroll)
	right := modernPanel("Live log", "", logScroll)
	body := container.NewHSplit(left, container.NewHSplit(center, right))
	body.SetOffset(0.28)
	bottom := container.NewBorder(nil, nil, nil, sendBtn, input)
	g.win.SetContent(appShell(container.NewBorder(head, bottom, nil, nil, body)))

	g.startRemoteLogStream(name, logBox, logScroll)

	// Reflect any Telegram host already running for this session (e.g. started
	// from the web or a previous GUI), so the toggle opens in the right state.
	go func() {
		ctx, cancel := bg(15)
		ts, err := g.remote.TelegramStatus(ctx, name)
		cancel()
		if err != nil || !ts.Hosting {
			return
		}
		fyne.Do(func() {
			setHosting(true)
			applyRemoteMode(st)
		})
	}()
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
					ts = e.Timestamp.Format("15:04") + "  "
				}
				appendLog(fmt.Sprintf("%s %s%s", engine.LogIcon(e.Type), ts, e.Message))
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

	// --- Server settings: full parity with the local Settings form — needs a
	// live connection. Mirrors cmd/thaimaturgy/settings.go field-for-field. ---
	provider := widget.NewSelect([]string{
		string(domain.ProviderOpenAI), string(domain.ProviderAnthropic),
		string(domain.ProviderGemini), string(domain.ProviderClaudeCLI),
	}, nil)
	model := widget.NewEntry()
	runModel := widget.NewEntry()
	runModel.SetPlaceHolder("(defaults to Model)")
	editModel := widget.NewEntry()
	editModel.SetPlaceHolder("(defaults to Model)")
	language := widget.NewSelect([]string{string(domain.LangEnglish), string(domain.LangSpanish)}, nil)
	importLang := widget.NewEntry()
	importLang.SetPlaceHolder("(follows UI language)")
	temperature := widget.NewEntry()
	maxTokens := widget.NewEntry()
	importTokens := widget.NewEntry()
	toolIter := widget.NewEntry()
	timeout := widget.NewEntry()
	autoSave := widget.NewCheck("", nil)
	autoSaveInterval := widget.NewEntry()
	ttsEnabled := widget.NewCheck("", nil)
	ttsVoice := widget.NewSelect([]string{
		string(domain.TTSVoiceAlloy), string(domain.TTSVoiceEcho), string(domain.TTSVoiceFable),
		string(domain.TTSVoiceOnyx), string(domain.TTSVoiceNova), string(domain.TTSVoiceShimmer),
	}, nil)
	spoilerGuard := widget.NewCheck("", nil)
	spoilerProvider := widget.NewSelect(spoilerProviderOptions(), nil)
	spoilerModel := widget.NewEntry()
	spoilerModel.SetPlaceHolder("(optional: blank = provider default)")
	openaiKey := widget.NewPasswordEntry()
	anthropicKey := widget.NewPasswordEntry()
	geminiKey := widget.NewPasswordEntry()
	telegramToken := widget.NewPasswordEntry()
	for _, e := range []*widget.Entry{openaiKey, anthropicKey, geminiKey, telegramToken} {
		e.SetPlaceHolder("(leave blank to keep the current value)")
	}
	telegramChat := widget.NewEntry()
	telegramChat.SetPlaceHolder("(optional: restrict the bot to one chat)")
	telegramUsers := widget.NewMultiLineEntry()
	telegramUsers.SetPlaceHolder("(optional: one numeric user id per line)")
	credLabel := widget.NewLabel("")
	credLabel.Wrapping = fyne.TextWrapWord

	srvForm := widget.NewForm(
		widget.NewFormItem("Detected credential", credLabel),
		widget.NewFormItem("Provider", provider),
		widget.NewFormItem("Model", model),
		widget.NewFormItem("Run model", runModel),
		widget.NewFormItem("Edit model", editModel),
		widget.NewFormItem("UI language", language),
		widget.NewFormItem("Import language", importLang),
		widget.NewFormItem("Temperature", temperature),
		widget.NewFormItem("Max tokens", maxTokens),
		widget.NewFormItem("Import max output tokens", importTokens),
		widget.NewFormItem("Oracle max tool iterations", toolIter),
		widget.NewFormItem("Request timeout (s)", timeout),
		widget.NewFormItem("Auto-save sessions", autoSave),
		widget.NewFormItem("Auto-save interval (s)", autoSaveInterval),
		widget.NewFormItem("TTS enabled", ttsEnabled),
		widget.NewFormItem("TTS voice", ttsVoice),
		widget.NewFormItem("Spoiler guard (Virtual DM)", spoilerGuard),
		widget.NewFormItem("Spoiler-guard provider", spoilerProvider),
		widget.NewFormItem("Spoiler-guard model", spoilerModel),
		widget.NewFormItem("OpenAI API key", openaiKey),
		widget.NewFormItem("Anthropic API key", anthropicKey),
		widget.NewFormItem("Gemini API key", geminiKey),
		widget.NewFormItem("Telegram bot token", telegramToken),
		widget.NewFormItem("Telegram chat id", telegramChat),
		widget.NewFormItem("Telegram allowed users", telegramUsers),
	)
	srvForm.Hide()
	srvStatus := widget.NewLabel("Loading server settings…")
	srvStatus.Wrapping = fyne.TextWrapWord
	var loaded *domain.Config
	saveSrv := widget.NewButtonWithIcon("Save server settings", theme.DocumentSaveIcon(), func() {
		if loaded == nil {
			return
		}
		loaded.Provider = domain.ProviderType(provider.Selected)
		loaded.Model = strings.TrimSpace(model.Text)
		loaded.RunModel = strings.TrimSpace(runModel.Text)
		loaded.EditModel = strings.TrimSpace(editModel.Text)
		loaded.Language = domain.Language(language.Selected)
		loaded.ImportLanguage = strings.TrimSpace(importLang.Text)
		loaded.Temperature = parseFloat(temperature.Text, loaded.Temperature)
		loaded.MaxTokens = parseInt(maxTokens.Text, loaded.MaxTokens)
		loaded.ImportMaxOutputTokens = parseInt(importTokens.Text, loaded.ImportMaxOutputTokens)
		loaded.OracleMaxToolIterations = parseInt(toolIter.Text, loaded.OracleMaxToolIterations)
		loaded.RequestTimeoutSeconds = parseInt(timeout.Text, loaded.RequestTimeoutSeconds)
		loaded.AutoSave = autoSave.Checked
		loaded.AutoSaveInterval = parseInt(autoSaveInterval.Text, loaded.AutoSaveInterval)
		loaded.TTS.Enabled = ttsEnabled.Checked
		loaded.TTS.Voice = domain.TTSVoice(ttsVoice.Selected)
		loaded.SpoilerGuard.Enabled = spoilerGuard.Checked
		loaded.SpoilerGuard.Provider = spoilerProviderValue(spoilerProvider.Selected)
		loaded.SpoilerGuard.Model = strings.TrimSpace(spoilerModel.Text)
		// Secrets are write-only: send only what was typed. getConfig blanked them,
		// so an untouched field stays "" and putConfig keeps the server's value.
		loaded.OpenAIAPIKey = strings.TrimSpace(openaiKey.Text)
		loaded.AnthropicAPIKey = strings.TrimSpace(anthropicKey.Text)
		loaded.GeminiAPIKey = strings.TrimSpace(geminiKey.Text)
		loaded.TelegramToken = strings.TrimSpace(telegramToken.Text)
		if s := strings.TrimSpace(telegramChat.Text); s == "" {
			loaded.TelegramChatID = 0
		} else if v, perr := strconv.ParseInt(s, 10, 64); perr == nil {
			loaded.TelegramChatID = v
		} else {
			g.showErr(fmt.Errorf("Telegram chat id must be a number (e.g. -1001234567890): %q", s))
			return
		}
		loaded.TelegramAllowedUsers = nil
		for _, line := range strings.Split(telegramUsers.Text, "\n") {
			if u := strings.TrimSpace(line); u != "" {
				loaded.TelegramAllowedUsers = append(loaded.TelegramAllowedUsers, u)
			}
		}
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
	)
	closeBar := container.NewHBox(widget.NewButton("Close", func() { pop.Hide() }))
	pop = widget.NewModalPopUp(
		container.NewPadded(container.NewBorder(nil, closeBar, nil, nil, container.NewVScroll(content))),
		g.win.Canvas(),
	)
	pop.Resize(fyne.NewSize(560, 680))
	pop.Show()

	// Load the server config (+ detected credential) in the background; on failure
	// the connection fields above still work, so the user can correct the URL/token
	// and reconnect. Capture the client (Apply & reconnect may swap g.remote).
	client := g.remote
	go func() {
		ctx, cancel := bg(15)
		defer cancel()
		cfg, authSource, err := client.ConfigWithAuth(ctx)
		fyne.Do(func() {
			if err != nil {
				srvStatus.SetText("Could not reach the server (" + err.Error() + "). Fix the URL/token above and Apply & reconnect.")
				return
			}
			loaded = cfg
			provider.SetSelected(string(cfg.Provider))
			model.SetText(cfg.Model)
			runModel.SetText(cfg.RunModel)
			editModel.SetText(cfg.EditModel)
			language.SetSelected(string(cfg.Language))
			importLang.SetText(cfg.ImportLanguage)
			temperature.SetText(strconv.FormatFloat(cfg.Temperature, 'g', -1, 64))
			maxTokens.SetText(strconv.Itoa(cfg.MaxTokens))
			importTokens.SetText(strconv.Itoa(cfg.ImportMaxOutputTokens))
			toolIter.SetText(strconv.Itoa(cfg.OracleMaxToolIterations))
			timeout.SetText(strconv.Itoa(cfg.RequestTimeoutSeconds))
			autoSave.SetChecked(cfg.AutoSave)
			autoSaveInterval.SetText(strconv.Itoa(cfg.AutoSaveInterval))
			ttsEnabled.SetChecked(cfg.TTS.Enabled)
			if cfg.TTS.Voice != "" {
				ttsVoice.SetSelected(string(cfg.TTS.Voice))
			}
			spoilerGuard.SetChecked(cfg.SpoilerGuard.Enabled)
			spoilerProvider.SetSelected(spoilerProviderLabel(cfg.SpoilerGuard.Provider))
			spoilerModel.SetText(cfg.SpoilerGuard.Model)
			if cfg.TelegramChatID != 0 {
				telegramChat.SetText(strconv.FormatInt(cfg.TelegramChatID, 10))
			}
			telegramUsers.SetText(strings.Join(cfg.TelegramAllowedUsers, "\n"))
			if authSource == "" {
				credLabel.SetText("(none detected)")
			} else {
				credLabel.SetText(authSource)
			}
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

// conversationMessages returns a session's persisted conversation messages.
func conversationMessages(st *domain.SessionState) []domain.Message {
	if st == nil || st.Conversation == nil {
		return nil
	}
	return st.Conversation.Messages
}

func layoutSpacer() fyne.CanvasObject { return widget.NewLabel("") }
