package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/mevdschee/fyne-authenticator/store"
	"github.com/mevdschee/pidfile"
	"github.com/skip2/go-qrcode"
	"github.com/xlzd/gotp"
	"github.com/zalando/go-keyring"
	"golang.design/x/clipboard"
)

const (
	Version      = "v1.0.0"
	AppID        = "com.tqdev.fyne-authenticator"
	WindowTitle  = "Fyne Authenticator"
	WindowWidth  = 500
	WindowHeight = 700
)

// runZbarCommand runs a zbar command with proper executable resolution and platform-specific arguments
func runZbarCommand(name string, args ...string) ([]byte, error) {
	var execName string
	var finalArgs []string

	if runtime.GOOS == "windows" {
		execName = name + ".exe"
		// Filter out unsupported arguments on Windows
		for _, arg := range args {
			if arg != "--nodbus" && arg != "--oneshot" {
				finalArgs = append(finalArgs, arg)
			}
		}
	} else {
		execName = name
		finalArgs = args
	}

	// Try to find the executable in PATH first
	execPath, err := exec.LookPath(execName)
	if err != nil {
		// If not found in PATH, try the current directory with explicit path
		if runtime.GOOS == "windows" {
			execPath = ".\\" + execName
		} else {
			execPath = "./" + execName
		}
	}

	return exec.Command(execPath, finalArgs...).Output()
}

var s *store.TotpStore

func main() {

	editMode := false

	a := app.NewWithID(AppID)

	// get password
	password, err := keyring.Get(AppID, "passphrase")
	if err != nil {
		// generate random passphrase
		password := gotp.RandomSecret(16)
		err := keyring.Set(AppID, "passphrase", password)
		if err != nil {
			log.Fatal(err)
		}
	}

	s = store.NewStore(path.Join(a.Storage().RootURI().Path(), "totp_tokens"), password)

	a.SetIcon(resourceIconPng)
	w := a.NewWindow(WindowTitle)
	w.Resize(fyne.NewSize(WindowWidth, WindowHeight))
	err = s.Load()
	if err != nil {
		dialog.NewError(err, w).Show()
		//p := widget.NewPasswordEntry()
		//d := dialog.NewForm("Login", "Ok", "", []*widget.FormItem{
		//	{Text: "Password", Widget: p},
		//}, func(b bool) {
		//	if !b {
		//		a.Quit()
		//	}
		//}, w)
		//d.Resize(fyne.NewSize(400, 200)) // Set minimum size of the dialog
		//d.SetOnClosed(func() {
		//	s.Password = p.Text
		//	err = s.Load()
		//	if err != nil {
		//		d.Show()
		//	}
		//})
		//d.Show()
	}

	pf := pidfile.New(AppID)
	pf.OnSecond = func(args []string) {
		w.Show()
		w.RequestFocus()
	}
	err = pf.Create()
	if err != nil {
		log.Fatal(err)
	}
	defer pf.Remove()
	if pf.FirstPid != os.Getpid() {
		a.Quit()
		return
	}

	label := widget.NewLabel("Select code to copy/paste")
	label.TextStyle = fyne.TextStyle{Italic: true}
	label.Alignment = fyne.TextAlignCenter
	add := widget.NewButtonWithIcon("", label.Theme().Icon(theme.IconNameContentAdd), nil)
	config := widget.NewButtonWithIcon("", label.Theme().Icon(theme.IconNameSettings), nil)
	export := widget.NewButtonWithIcon("", label.Theme().Icon(theme.IconNameDownload), nil)
	export.Hide()
	done := widget.NewButtonWithIcon("", label.Theme().Icon(theme.IconNameConfirm), nil)
	done.Hide()
	header := container.NewBorder(nil, nil, container.NewVBox(config, done), container.NewVBox(add, export), label)

	onClose := func() {
		if editMode {
			done.OnTapped()
		}
		w.Hide()
	}

	buildMenu := func(m *fyne.Menu) {
		if m == nil {
			return
		}
		m.Items = []*fyne.MenuItem{}
		m.Items = append(m.Items, fyne.NewMenuItem("Show", func() {
			w.Show()
		}))
		m.Items = append(m.Items, fyne.NewMenuItemSeparator())
		for _, e := range s.Entries {
			name := e.Issuer + ": " + e.Name
			if e.Issuer == "" {
				name = e.Name
			}
			m.Items = append(m.Items, fyne.NewMenuItem(name, func() {
				clipboard.Write(clipboard.FmtText, []byte(gotp.NewDefaultTOTP(e.Secret).Now()))
			}))
		}
		m.Items = append(m.Items, fyne.NewMenuItemSeparator())
		m.Items = append(m.Items, fyne.NewMenuItem("Quit", func() { onClose(); a.Quit() }))
	}

	updateMenu := func() {
	}

	var systemTrayMenu *fyne.Menu

	// start system tray
	if d, ok := a.(desktop.App); ok {
		w.SetCloseIntercept(onClose)
		d.SetSystemTrayIcon(resourceIconPng)
		systemTrayMenu = fyne.NewMenu("Authenticator")
		updateMenu = func() {
			buildMenu(systemTrayMenu)
			systemTrayMenu.Refresh()
		}
		updateMenu()
		d.SetSystemTrayMenu(systemTrayMenu)
		//w.Show() // comment out to start hidden
	} else {
		w.Show()
	}

	var updateList sync.Mutex
	var selectedListItem = -1
	list := widget.NewList(
		func() int { return len(s.Entries) },
		func() fyne.CanvasObject {
			name := widget.NewLabel("Company")
			up := widget.NewButtonWithIcon("", name.Theme().Icon(theme.IconNameMoveUp), nil)
			down := widget.NewButtonWithIcon("", name.Theme().Icon(theme.IconNameMoveDown), nil)
			delete := widget.NewButtonWithIcon("", name.Theme().Icon(theme.IconNameDelete), nil)
			rowButtons := container.NewHBox(up, down, delete)
			progress := widget.NewProgressBar()
			progress.Max = 30
			progress.Min = 1
			hbox := container.NewHBox(progress, rowButtons)

			return container.NewBorder(nil, nil, name, hbox)
		},
		nil,
	)

	list.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) {
		updateList.Lock()
		name := s.Entries[i].Issuer + ": " + s.Entries[i].Name
		if s.Entries[i].Issuer == "" {
			name = s.Entries[i].Name
		}
		totp := gotp.NewDefaultTOTP(s.Entries[i].Secret)
		code, t := totp.NowWithExpiration()
		container := o.(*fyne.Container)
		nameWidget := container.Objects[0].(*widget.Label)
		progressWidget := container.Objects[1].(*fyne.Container).Objects[0].(*widget.ProgressBar)
		rowButtons := container.Objects[1].(*fyne.Container).Objects[1].(*fyne.Container)
		if editMode {
			progressWidget.Hide()
			rowButtons.Show()
		} else {
			progressWidget.Show()
			rowButtons.Hide()
		}
		moveUp := rowButtons.Objects[0].(*widget.Button)
		moveDown := rowButtons.Objects[1].(*widget.Button)
		delete := rowButtons.Objects[2].(*widget.Button)
		moveUp.OnTapped = func() {
			if i > 0 {
				s.Entries[i], s.Entries[i-1] = s.Entries[i-1], s.Entries[i]
				list.Refresh()
				updateMenu()
			}
		}
		moveDown.OnTapped = func() {
			if i < len(s.Entries)-1 {
				s.Entries[i], s.Entries[i+1] = s.Entries[i+1], s.Entries[i]
				list.Refresh()
				updateMenu()
			}
		}
		delete.OnTapped = func() {
			s.Entries = append(s.Entries[0:i], s.Entries[i+1:]...)
			list.Refresh()
			updateMenu()
		}
		if selectedListItem == i {
			nameWidget.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
			nameWidget.SetText(name + " (copied)")
		} else {
			nameWidget.TextStyle = fyne.TextStyle{Bold: false, Italic: false}
			nameWidget.SetText(name)
		}
		width := nameWidget.MinSize().Width
		maxWidth := container.Size().Width
		if editMode {
			maxWidth -= rowButtons.Size().Width
		} else {
			maxWidth -= progressWidget.Size().Width
		}
		if width > maxWidth {
			for width > maxWidth {
				name = name[:len(name)-1]
				nameWidget.SetText(name + "...")
				width = nameWidget.MinSize().Width
			}
		}
		//log.Printf("%v,%v,%v,%v", code, t, time.Now().Unix(), t-time.Now().Unix())
		progressWidget.SetValue(float64((t - time.Now().Unix())))
		progressWidget.TextFormatter = func() string {
			return code
		}
		progressWidget.Refresh()
		updateList.Unlock()
	}
	go func() {
		for {
			time.Sleep(time.Second)
			list.Refresh()
		}
	}()
	go func() {
		width := w.Canvas().Size().Width
		height := w.Canvas().Size().Height
		for {
			time.Sleep(time.Millisecond * 125)
			if w.Content().Visible() {
				newWidth := w.Canvas().Size().Width
				newHeight := w.Canvas().Size().Height
				if width != newWidth || height != newHeight {
					width = newWidth
					height = newHeight
					list.Refresh()
				}
			}
		}
	}()

	list.OnUnselected = func(id widget.ListItemID) {
		selectedListItem = -1
		list.Refresh()
	}
	list.OnSelected = func(id widget.ListItemID) {
		go func() {
			if editMode {
				nameEntry := widget.NewEntry()
				nameEntry.SetText(s.Entries[id].Name)
				issuerEntry := widget.NewEntry()
				issuerEntry.SetText(s.Entries[id].Issuer)
				name := s.Entries[id].Issuer + ": " + s.Entries[id].Name
				if s.Entries[id].Issuer == "" {
					name = s.Entries[id].Name
				}
				form := &widget.Form{
					Items: []*widget.FormItem{
						{Text: "Issuer", Widget: issuerEntry},
						{Text: "Name", Widget: nameEntry},
					},
					OnSubmit: func() {
						s.Entries[id].Name = nameEntry.Text
						s.Entries[id].Issuer = issuerEntry.Text
					},
				}
				d := dialog.NewForm(name+" (edit)", "Ok", "Cancel", form.Items, func(b bool) {
					if b {
						form.OnSubmit()
					}
				}, w)
				d.Resize(fyne.NewSize(400, 200)) // Set minimum size of the dialog
				d.Show()
				list.Unselect(id)
			} else {
				otp := gotp.NewDefaultTOTP(s.Entries[id].Secret)
				code, _ := otp.NowWithExpiration()
				clipboard.Write(clipboard.FmtText, []byte(code))
				selectedListItem = id
				list.Refresh()
				time.Sleep(time.Millisecond * 750)
				list.Unselect(id)
			}
		}()
	}

	config.OnTapped = func() {
		editMode = true
		list.Refresh()
		label.Text = "Select code to edit name"
		label.Refresh()
		config.Hide()
		export.Show()
		add.Hide()
		done.Show()
	}
	done.OnTapped = func() {
		updateMenu()
		s.Save()
		editMode = false
		list.Refresh()
		label.Text = "Select code to copy/paste"
		label.Refresh()
		config.Show()
		export.Hide()
		add.Show()
		done.Hide()
	}
	w.SetOnDropped(func(p fyne.Position, files []fyne.URI) {
		for _, file := range files {
			buf, err := runZbarCommand("zbarimg", "--quiet", "--oneshot", file.String())
			if err != nil {
				if err.Error() == "exit status 4" {
					dialog.NewError(errors.New("no QR code found"), w).Show()
					continue
				}
				dialog.NewError(err, w).Show()
				continue
			}
			url := strings.TrimSpace(string(buf))
			if !strings.HasPrefix(url, "QR-Code:") {
				dialog.NewError(errors.New("no QR code found"), w).Show()
				continue
			}
			url = strings.TrimPrefix(url, "QR-Code:")
			err = s.AddUrl(url)
			if err != nil {
				dialog.NewError(err, w).Show()
				continue
			}
			list.Refresh()
			updateMenu()
			err = s.Save()
			if err != nil {
				dialog.NewError(err, w).Show()
				continue
			}
		}
	})
	w.Canvas().AddShortcut(&fyne.ShortcutPaste{Clipboard: w.Clipboard()}, func(shortcut fyne.Shortcut) {
		// try to paste clipboard contents as URL
		buf := clipboard.Read(clipboard.FmtText)
		url := strings.TrimSpace(string(buf))
		err = s.AddUrl(url)
		if err == nil {
			list.Refresh()
			updateMenu()
			err = s.Save()
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			return
		}
		// try to paste clipboard contents as image
		buf = clipboard.Read(clipboard.FmtImage)
		if buf == nil {
			return
		}
		tmpfile, err := os.CreateTemp("", "qr-*.png")
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
		defer os.Remove(tmpfile.Name())
		_, err = tmpfile.Write(buf)
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
		tmpfile.Close()
		buf, err = runZbarCommand("zbarimg", "--quiet", "--oneshot", tmpfile.Name())
		if err != nil {
			if err.Error() == "exit status 4" {
				dialog.NewError(errors.New("no QR code found"), w).Show()
				return
			}
			dialog.NewError(err, w).Show()
			return
		}
		url = strings.TrimSpace(string(buf))
		if !strings.HasPrefix(url, "QR-Code:") {
			dialog.NewError(errors.New("no QR code found"), w).Show()
			return
		}
		url = strings.TrimPrefix(url, "QR-Code:")
		err = s.AddUrl(url)
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
		list.Refresh()
		updateMenu()
		err = s.Save()
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
	})
	add.OnTapped = func() {
		found := false
		url := ""

		if !found {
			for i := 0; i < 10; i++ {
				buf, err := runZbarCommand("zbarcam", "--quiet", "--nodbus", "--oneshot", fmt.Sprintf("/dev/video%d", i))
				if err != nil {
					if err.Error() == "exit status 1" {
						continue
					}
					dialog.NewError(err, w).Show()
					break
				}
				found = true
				url = strings.TrimSpace(string(buf))
				break
			}
		}
		if !found {
			dialog.NewError(fmt.Errorf("no webcam found"), w).Show()
			return
		}
		if url == "" {
			return
		}
		if !strings.HasPrefix(url, "QR-Code:") {
			dialog.NewError(errors.New("no QR code found"), w).Show()
			return
		}
		url = strings.TrimPrefix(url, "QR-Code:")
		err = s.AddUrl(url)
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
		list.Refresh()
		updateMenu()
		err = s.Save()
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
	}
	export.OnTapped = func() {
		buf := bytes.NewBuffer([]byte{})
		zipWriter := zip.NewWriter(buf)
		for _, e := range s.Entries {
			re := regexp.MustCompile(`[^\w.-]`)
			name := re.ReplaceAllLiteralString(e.Name+" "+e.Issuer, "_")
			fileName := filepath.Join(strings.ToLower(name) + ".png")
			header := &zip.FileHeader{
				Name:     fileName,
				Method:   zip.Deflate,
				Modified: time.Now(),
			}
			entryWriter, err := zipWriter.CreateHeader(header)
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			url := gotp.BuildUri("totp", e.Secret, e.Name, e.Issuer, "", 0, 6, 30)
			b, err := qrcode.Encode(url, qrcode.Medium, -3)
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			fileReader := bufio.NewReader(bytes.NewReader(b))
			_, err = io.Copy(entryWriter, fileReader)
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			zipWriter.Flush()
		}
		zipWriter.Close()
		dialog.ShowFileSave(func(r fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			if r == nil {
				return
			}
			defer r.Close()
			_, err = r.Write(buf.Bytes())
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
		}, w)
	}

	rootContainer := container.NewBorder(nil, header, nil, nil, list)

	w.SetContent(rootContainer)

	a.Run()
}
