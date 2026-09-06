package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// ================= KONFIGURASI COOKIES =================

// igCookiesFile adalah path ke file cookies Instagram format Netscape
// (cookies.txt), hasil export dari browser (mis. pakai extension
// "Get cookies.txt LOCALLY") saat login ke instagram.com.
// Taruh file ini di direktori kerja bot. Kalau file tidak ada,
// semua fungsi di bawah otomatis jalan tanpa cookies seperti biasa.
const igCookiesFile = "ig_cookies.txt"

// igCookiesFileExists mengecek apakah file cookies tersedia.
func igCookiesFileExists() bool {
	_, err := os.Stat(igCookiesFile)
	return err == nil
}

// igLoadCookieHeader membaca file cookies.txt format Netscape dan
// mengubahnya jadi string siap pakai untuk header HTTP "Cookie".
func igLoadCookieHeader() (string, error) {
	data, err := os.ReadFile(igCookiesFile)
	if err != nil {
		return "", err
	}

	var pairs []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			// Baris #HttpOnly_domain masih valid, sisanya komentar biasa.
			if strings.HasPrefix(line, "#HttpOnly_") {
				line = strings.TrimPrefix(line, "#HttpOnly_")
			} else {
				continue
			}
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		name := fields[5]
		value := fields[6]
		if name == "" {
			continue
		}
		pairs = append(pairs, name+"="+value)
	}

	if len(pairs) == 0 {
		return "", fmt.Errorf("tidak ada cookie valid ditemukan di %s", igCookiesFile)
	}

	return strings.Join(pairs, "; "), nil
}

// igSetCookieHeader memasang header Cookie ke request kalau file cookies
// tersedia. Kegagalan membaca cookies tidak dianggap fatal (diam-diam
// lanjut tanpa cookies), supaya fallback lama tetap jalan.
func igSetCookieHeader(req *http.Request) {
	if !igCookiesFileExists() {
		return
	}
	cookieHeader, err := igLoadCookieHeader()
	if err != nil {
		return
	}
	req.Header.Set("Cookie", cookieHeader)
}

// ================= INIT =================

func init() {
	Register(Plugin{
		Command: "ig",
		Desc:    "Download postingan Instagram (foto/video/reels) dari link",
		Run:     igDownload,
	})

	Register(Plugin{
		Command: "igpp",
		Desc:    "Download foto profil Instagram (contoh: !igpp username)",
		Run:     igProfilePicture,
	})
}

// ================= POST (FOTO / VIDEO / REELS) =================

func igDownload(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!ig https://www.instagram.com/p/xxxxxxxx/")
		return
	}

	url := args[0]
	if !strings.Contains(url, "instagram.com") {
		reply(client, m, "❌ Link itu bukan link Instagram yang valid.")
		return
	}

	reactProcessing(client, m)

	fileBase := fmt.Sprintf("ig_%s", m.Info.ID)
	defer func() {
		matches, _ := filepath.Glob(fileBase + "*")
		for _, f := range matches {
			os.Remove(f)
		}
	}()

	// --- Percobaan 1-3: yt-dlp dengan beberapa variasi app_id ---
	// --extractor-args instagram:app_id=ios/web: workaround bug yt-dlp
	// dimana post berisi foto/carousel gagal dengan "No video formats found".
	// --cookies dipasang otomatis kalau ig_cookies.txt tersedia, supaya
	// yt-dlp mengakses sebagai user yang login (jauh lebih stabil).
	var out []byte
	var err error
	var matches []string

	tryYtdlp := func(extraArgs ...string) {
		baseArgs := []string{
			"--no-playlist", "--no-part",
			"--retries", "10", "--fragment-retries", "10",
			"-o", fileBase + ".%(ext)s",
		}
		if igCookiesFileExists() {
			baseArgs = append(baseArgs, "--cookies", igCookiesFile)
		}
		fullArgs := append(baseArgs, extraArgs...)
		fullArgs = append(fullArgs, url)
		cmd := exec.Command("yt-dlp", fullArgs...)
		o, e := cmd.CombinedOutput()
		out = append(out, o...)
		if e == nil {
			err = nil
		} else if err == nil {
			err = e
		}
		matches, _ = filepath.Glob(fileBase + ".*")
	}

	tryYtdlp("--extractor-args", "instagram:app_id=ios")
	if len(matches) == 0 {
		tryYtdlp("--skip-download", "--write-thumbnail", "--extractor-args", "instagram:app_id=ios")
	}
	if len(matches) == 0 {
		tryYtdlp("--extractor-args", "instagram:app_id=web")
	}

	// --- Percobaan 4: gallery-dl (library berbeda dari yt-dlp) ---
	// Kelebihan gallery-dl: bisa dapat SEMUA foto di carousel sekaligus,
	// bukan cuma satu. Perlu diinstall dulu: pip install gallery-dl --break-system-packages
	// --cookies juga dipasang otomatis di sini kalau tersedia.
	if len(matches) == 0 {
		gdlFiles, gdlErr := igGalleryDLDownload(url, fileBase)
		if gdlErr == nil && len(gdlFiles) > 0 {
			sendAllMedia(client, m, gdlFiles)
			return
		}
		if gdlErr != nil {
			out = append(out, []byte("\n[gallery-dl] "+gdlErr.Error())...)
		}
	}

	// --- Percobaan 5: embed page Instagram (bisa dapat carousel) ---
	// Halaman /embed/captioned/ bersifat publik dan tidak butuh login.
	// Biasanya masih menyimpan URL media di dalam script JSON walau
	// endpoint API utama IG sedang diblokir. Cookie tetap dipasang kalau
	// ada, untuk jaga-jaga post yang butuh sesi login.
	var embedErrMsg string
	if len(matches) == 0 {
		embedFiles, embedErr := igEmbedFallback(url, fileBase)
		if embedErr == nil && len(embedFiles) > 0 {
			sendAllMedia(client, m, embedFiles)
			return
		}
		if embedErr != nil {
			embedErrMsg = embedErr.Error()
			out = append(out, []byte("\n[embed] "+embedErr.Error())...)
		}
	}

	// --- Percobaan 6: scrape og:image/og:video langsung dari halaman ---
	var scrapeErrMsg string
	if len(matches) == 0 {
		scrapedFile, scrapeErr := igScrapeFallback(url, fileBase)
		if scrapeErr == nil && scrapedFile != "" {
			matches = []string{scrapedFile}
			err = nil
		} else if scrapeErr != nil {
			scrapeErrMsg = scrapeErr.Error()
		}
	}

	if len(matches) == 0 {
		reactError(client, m)
		msg := "❌ Gagal mengunduh dari Instagram (sudah coba yt-dlp, gallery-dl, embed page, dan scrape langsung).\n"
		if !igCookiesFileExists() {
			msg += "ℹ️ Belum ada file " + igCookiesFile + " — tambahkan cookies login IG supaya lebih stabil.\n"
		}
		if err != nil {
			msg += "Error yt-dlp: " + err.Error() + "\n"
		}
		if embedErrMsg != "" {
			msg += "Error fallback embed: " + embedErrMsg + "\n"
		}
		if scrapeErrMsg != "" {
			msg += "Error fallback scrape: " + scrapeErrMsg + "\n"
		}
		msg += "Log: " + string(out)
		reply(client, m, msg)
		return
	}

	sendAllMedia(client, m, matches)
}

// sendAllMedia mengirim satu atau lebih file media (gambar/video) ke chat,
// masing-masing sebagai pesan terpisah, lalu bereaksi ✅ di akhir.
func sendAllMedia(client *whatsmeow.Client, m *events.Message, files []string) {
	ctx := context.Background()
	sentAny := false

	for _, resultFile := range files {
		data, err := os.ReadFile(resultFile)
		if err != nil {
			continue
		}
		ext := strings.ToLower(resultFile[strings.LastIndex(resultFile, ".")+1:])

		switch ext {
		case "mp4", "mov", "mkv", "webm":
			uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
			if errUpload != nil {
				continue
			}
			client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
				VideoMessage: &waProto.VideoMessage{
					URL:           &uploaded.URL,
					DirectPath:    &uploaded.DirectPath,
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    &uploaded.FileLength,
					Mimetype:      StringPtr("video/mp4"),
				},
			})
			sentAny = true

		case "jpg", "jpeg", "png", "webp":
			uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
			if errUpload != nil {
				continue
			}
			client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
				ImageMessage: &waProto.ImageMessage{
					URL:           &uploaded.URL,
					DirectPath:    &uploaded.DirectPath,
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    &uploaded.FileLength,
					Mimetype:      StringPtr("image/jpeg"),
				},
			})
			sentAny = true
		}
	}

	if sentAny {
		reactDone(client, m)
	} else {
		reactError(client, m)
		reply(client, m, "❌ Ditemukan file, tapi gagal mengunggah/mengirim ke WhatsApp.")
	}
}

// igGalleryDLDownload mencoba mengunduh post Instagram pakai gallery-dl,
// library terpisah dari yt-dlp yang seringkali lebih baik menangani post
// foto/carousel Instagram. Mengembalikan daftar path semua file media yang
// berhasil diunduh (bisa lebih dari satu untuk carousel).
func igGalleryDLDownload(url string, fileBase string) ([]string, error) {
	tmpDir := fileBase + "_gdl"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	cmdArgs := []string{"--dest", tmpDir}
	if igCookiesFileExists() {
		cmdArgs = append(cmdArgs, "--cookies", igCookiesFile)
	}
	cmdArgs = append(cmdArgs, url)

	cmd := exec.Command("gallery-dl", cmdArgs...)
	out, err := cmd.CombinedOutput()

	var mediaFiles []string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".mp4", ".mov", ".mkv", ".webm":
			// Salin ke lokasi permanen dengan nama fileBase_N.ext supaya
			// tidak ikut terhapus saat tmpDir dibersihkan.
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			newPath := fmt.Sprintf("%s_%d%s", fileBase, len(mediaFiles)+1, ext)
			if writeErr := os.WriteFile(newPath, data, 0644); writeErr == nil {
				mediaFiles = append(mediaFiles, newPath)
			}
		}
		return nil
	})

	if len(mediaFiles) == 0 {
		if err != nil {
			return nil, fmt.Errorf("gallery-dl gagal: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil, fmt.Errorf("gallery-dl tidak menghasilkan file media")
	}

	return mediaFiles, nil
}

// ================= EMBED PAGE FALLBACK =================

var igShortcodeRegex = regexp.MustCompile(`instagram\.com/(?:p|reel|reels)/([A-Za-z0-9_-]+)`)
var igEmbedVideoURLRegex = regexp.MustCompile(`"video_url":"([^"]+)"`)
var igEmbedDisplayURLRegex = regexp.MustCompile(`"display_url":"([^"]+)"`)

// igEmbedFallback mengambil media dari halaman embed Instagram
// (https://www.instagram.com/p/{shortcode}/embed/captioned/), yang publik
// dan tidak butuh login. Halaman ini biasanya masih menyimpan URL media
// di dalam script JSON walau endpoint API utama IG sedang diblokir.
// Bisa dapat SEMUA foto carousel sekaligus, mirip gallery-dl.
func igEmbedFallback(postURL string, fileBase string) ([]string, error) {
	scMatch := igShortcodeRegex.FindStringSubmatch(postURL)
	if len(scMatch) < 2 {
		return nil, fmt.Errorf("tidak bisa mengekstrak shortcode dari URL")
	}
	shortcode := scMatch[1]
	embedURL := "https://www.instagram.com/p/" + shortcode + "/embed/captioned/"

	req, err := http.NewRequest("GET", embedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	igSetCookieHeader(req)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	unescape := func(raw string) string {
		unquoted, uerr := strconv.Unquote(`"` + raw + `"`)
		if uerr != nil {
			// fallback manual kalau gagal unquote (mis. ada karakter aneh)
			return strings.ReplaceAll(raw, `\/`, "/")
		}
		return unquoted
	}

	download := func(mediaURL string, idx int, ext string) (string, error) {
		mReq, mErr := http.NewRequest("GET", mediaURL, nil)
		if mErr != nil {
			return "", mErr
		}
		mReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
		igSetCookieHeader(mReq)

		mResp, mErr := httpClient.Do(mReq)
		if mErr != nil {
			return "", mErr
		}
		defer mResp.Body.Close()

		data, mErr := io.ReadAll(mResp.Body)
		if mErr != nil {
			return "", mErr
		}

		outPath := fmt.Sprintf("%s_%d.%s", fileBase, idx, ext)
		if wErr := os.WriteFile(outPath, data, 0644); wErr != nil {
			return "", wErr
		}
		return outPath, nil
	}

	// Prioritas: video (reels/video post) dulu.
	if vidMatch := igEmbedVideoURLRegex.FindStringSubmatch(html); len(vidMatch) >= 2 {
		videoURL := unescape(vidMatch[1])
		path, dErr := download(videoURL, 1, "mp4")
		if dErr != nil {
			return nil, dErr
		}
		return []string{path}, nil
	}

	// Kalau tidak ada video, ambil semua display_url (bisa banyak untuk carousel).
	imgMatches := igEmbedDisplayURLRegex.FindAllStringSubmatch(html, -1)
	if len(imgMatches) == 0 {
		return nil, fmt.Errorf("tidak ditemukan video_url maupun display_url di halaman embed")
	}

	seen := make(map[string]bool)
	var files []string
	idx := 1
	for _, im := range imgMatches {
		imgURL := unescape(im[1])
		if seen[imgURL] {
			continue
		}
		seen[imgURL] = true

		path, dErr := download(imgURL, idx, "jpg")
		if dErr != nil {
			continue
		}
		files = append(files, path)
		idx++
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("gagal mengunduh media dari display_url yang ditemukan")
	}

	return files, nil
}

// ================= FOTO PROFIL =================

var igOgImageRegex = regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
var igOgVideoRegex = regexp.MustCompile(`<meta property="og:video" content="([^"]+)"`)

// igScrapeFallback mengambil media (video kalau ada, kalau tidak ambil foto)
// langsung dari meta tag halaman post Instagram, tanpa lewat yt-dlp sama
// sekali. Mengembalikan path file yang berhasil disimpan (fileBase + ekstensi).
func igScrapeFallback(postURL string, fileBase string) (string, error) {
	req, err := http.NewRequest("GET", postURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	igSetCookieHeader(req)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(body)

	var mediaURL string
	var ext string
	if vidMatch := igOgVideoRegex.FindStringSubmatch(html); len(vidMatch) >= 2 {
		mediaURL = strings.ReplaceAll(vidMatch[1], "&amp;", "&")
		ext = "mp4"
	} else if imgMatch := igOgImageRegex.FindStringSubmatch(html); len(imgMatch) >= 2 {
		mediaURL = strings.ReplaceAll(imgMatch[1], "&amp;", "&")
		ext = "jpg"
	} else {
		return "", fmt.Errorf("tidak ditemukan og:video maupun og:image di halaman")
	}

	mediaReq, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		return "", err
	}
	mediaReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	igSetCookieHeader(mediaReq)

	mediaResp, err := httpClient.Do(mediaReq)
	if err != nil {
		return "", err
	}
	defer mediaResp.Body.Close()

	data, err := io.ReadAll(mediaResp.Body)
	if err != nil {
		return "", err
	}

	outPath := fileBase + "." + ext
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

func igProfilePicture(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!igpp username")
		return
	}

	username := strings.TrimPrefix(strings.TrimSpace(args[0]), "@")
	reactProcessing(client, m)

	profileURL := "https://www.instagram.com/" + username + "/"

	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membuat request.")
		return
	}
	// User-Agent browser biasa, supaya tidak langsung ditolak Instagram.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	igSetCookieHeader(req)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal mengakses Instagram: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membaca halaman profil.")
		return
	}

	matches := igOgImageRegex.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		reactError(client, m)
		reply(client, m, "❌ Tidak bisa menemukan foto profil. Kemungkinan username salah, akun private, atau Instagram sedang membatasi akses tanpa login.")
		return
	}
	imageURL := strings.ReplaceAll(matches[1], "&amp;", "&")

	imgReq, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membuat request gambar.")
		return
	}
	imgReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	igSetCookieHeader(imgReq)

	imgResp, err := httpClient.Do(imgReq)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal mengunduh foto profil.")
		return
	}
	defer imgResp.Body.Close()

	data, err := io.ReadAll(imgResp.Body)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membaca data foto profil.")
		return
	}

	ctx := context.Background()
	uploaded, err := client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal upload foto profil ke WhatsApp.")
		return
	}

	caption := "📸 Foto profil @" + username
	client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
			Mimetype:      StringPtr("image/jpeg"),
			Caption:       &caption,
		},
	})

	reactDone(client, m)
}
