package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// File/Plugin manager commands (owner-only)
// Commands:
//  - .pluginadd <filename.go> (REPLY with code)    -> create new plugin file under plugin/
//  - .pluginedit <filename.go> (REPLY with code)   -> overwrite existing plugin file (creates backup)
//  - .pluginrm <filename.go>                       -> delete plugin file (moves to .trash with timestamp)
//  - .mkdir <relative/path>                        -> create directory (relative to repo)
//  - .mkfile <relative/path/file> (REPLY with content) -> create file (intermediate dirs created)
// Notes:
//  - All paths must be relative, must not contain ".." or start with '/'.
//  - For plugin commands, path is forced into plugin/ directory.
//  - Only owner can run these commands (uses isOwner defined elsewhere in package).

func init() {
	Register(Plugin{
		Command: "pluginadd",
		Desc:    "[Owner] Tambah plugin baru: balas pesan berisi kode dan gunakan .pluginadd nama_file.go",
		Run:     cmdPluginAdd,
	})
	Register(Plugin{
		Command: "pluginedit",
		Desc:    "[Owner] Edit plugin yang ada: balas pesan berisi kode dan gunakan .pluginedit nama_file.go",
		Run:     cmdPluginEdit,
	})
	Register(Plugin{
		Command: "pluginrm",
		Desc:    "[Owner] Hapus plugin: .pluginrm nama_file.go (dipindahkan ke .trash)",
		Run:     cmdPluginRemove,
	})
	Register(Plugin{
		Command: "mkdir",
		Desc:    "[Owner] Buat folder relatif: .mkdir path/to/folder",
		Run:     cmdMkdir,
	})
	Register(Plugin{
		Command: "mkfile",
		Desc:    "[Owner] Buat file baru: balas pesan berisi isi dan gunakan .mkfile path/to/file.ext",
		Run:     cmdMkFile,
	})
}

// helpers
func sanitizeRelPath(p string) (string, error) {
	p = filepath.Clean(p)
	// Disallow absolute
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be relative, tidak boleh diawali /")
	}
	// Disallow parent traversal
	if strings.HasPrefix(p, "..") || strings.Contains(p, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("path tidak boleh berisi '..' (parent traversal)")
	}
	return p, nil
}

func requireReplyText(m *events.Message) (string, error) {
	if m.Message.GetExtendedTextMessage() == nil || m.Message.GetExtendedTextMessage().GetContextInfo() == nil {
		return "", fmt.Errorf("harap balas (reply) pesan yang berisi isi file")
	}
	quoted := m.Message.GetExtendedTextMessage().GetContextInfo().QuotedMessage
	if quoted == nil {
		return "", fmt.Errorf("pesan balasan tidak mengandung teks yang valid")
	}
	// Grab text from quoted message (conversation or extended)
	text := quoted.GetConversation()
	if text == "" && quoted.GetExtendedTextMessage() != nil {
		text = quoted.GetExtendedTextMessage().GetText()
	}
	if text == "" {
		return "", fmt.Errorf("konten balasan kosong")
	}
	return text, nil
}

// Command implementations
func cmdPluginAdd(client *whatsmeow.Client, m *events.Message, args []string) {
	if !isOwner(m) {
		reply(client, m, "❌ Perintah ini khusus owner bot.")
		return
	}
	if len(args) == 0 {
		reply(client, m, "❌ Gunakan: .pluginadd nama_plugin.go (balas pesan yang berisi kode)")
		return
	}
	name := args[0]
	if !strings.HasSuffix(name, ".go") {
		reply(client, m, "❌ Nama plugin harus berekstensi .go")
		return
	}
	cleanName := filepath.Base(name) // prevent subdirs here
	p := filepath.Join("plugin", cleanName)

	if _, err := os.Stat(p); err == nil {
		reply(client, m, "⚠️ File sudah ada: " + p)
		return
	}

	content, err := requireReplyText(m)
	if err != nil {
		reply(client, m, "❌ " + err.Error())
		return
	}

	// Create plugin dir if not exists
	if err := os.MkdirAll("plugin", 0o755); err != nil {
		reply(client, m, "❌ Gagal membuat folder plugin: "+err.Error())
		return
	}

	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		reply(client, m, "❌ Gagal menulis file: "+err.Error())
		return
	}

	reply(client, m, "✅ Plugin dibuat: "+p+" — restart bot agar plugin aktif")
}

func cmdPluginEdit(client *whatsmeow.Client, m *events.Message, args []string) {
	if !isOwner(m) {
		reply(client, m, "❌ Perintah ini khusus owner bot.")
		return
	}
	if len(args) == 0 {
		reply(client, m, "❌ Gunakan: .pluginedit nama_plugin.go (balas pesan yang berisi kode)")
		return
	}
	name := args[0]
	if !strings.HasSuffix(name, ".go") {
		reply(client, m, "❌ Nama plugin harus berekstensi .go")
		return
	}
	cleanName := filepath.Base(name)
	p := filepath.Join("plugin", cleanName)

	if _, err := os.Stat(p); os.IsNotExist(err) {
		reply(client, m, "⚠️ File tidak ditemukan: " + p)
		return
	}

	content, err := requireReplyText(m)
	if err != nil {
		reply(client, m, "❌ " + err.Error())
		return
	}

	// Backup existing file
	bakDir := ".trash"
	if err := os.MkdirAll(bakDir, 0o755); err == nil {
		timestamp := time.Now().Format("20060102_150405")
		bakName := fmt.Sprintf("%s.%s.bak", cleanName, timestamp)
		bakPath := filepath.Join(bakDir, bakName)
		if err := os.Rename(p, bakPath); err != nil {
			// if rename fails, try copy
			orig, _ := os.ReadFile(p)
			_ = os.WriteFile(bakPath, orig, 0644)
		}
	}

	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		reply(client, m, "❌ Gagal menulis file: "+err.Error())
		return
	}

	reply(client, m, "✅ Plugin di-update: "+p+" (backup ada di .trash)")
}

func cmdPluginRemove(client *whatsmeow.Client, m *events.Message, args []string) {
	if !isOwner(m) {
		reply(client, m, "❌ Perintah ini khusus owner bot.")
		return
	}
	if len(args) == 0 {
		reply(client, m, "❌ Gunakan: .pluginrm nama_plugin.go")
		return
	}
	name := args[0]
	if !strings.HasSuffix(name, ".go") {
		reply(client, m, "❌ Nama plugin harus berekstensi .go")
		return
	}
	cleanName := filepath.Base(name)
	p := filepath.Join("plugin", cleanName)

	if _, err := os.Stat(p); os.IsNotExist(err) {
		reply(client, m, "⚠️ File tidak ditemukan: " + p)
		return
	}

	// move to .trash with timestamp
	bakDir := ".trash"
	if err := os.MkdirAll(bakDir, 0o755); err != nil {
		reply(client, m, "❌ Gagal membuat folder trash: "+err.Error())
		return
	}
	timestamp := time.Now().Format("20060102_150405")
	bakName := fmt.Sprintf("%s.%s.removed", cleanName, timestamp)
	bakPath := filepath.Join(bakDir, bakName)
	if err := os.Rename(p, bakPath); err != nil {
		// try copy+remove if rename fails
		orig, errr := os.ReadFile(p)
		if errr != nil {
			reply(client, m, "❌ Gagal memindahkan file: "+err.Error())
			return
		}
		if errw := os.WriteFile(bakPath, orig, 0644); errw != nil {
			reply(client, m, "❌ Gagal membuat backup: "+errw.Error())
			return
		}
		if err := os.Remove(p); err != nil {
			reply(client, m, "❌ Gagal menghapus file asli: "+err.Error())
			return
		}
	}

	reply(client, m, "✅ Plugin dipindahkan ke .trash: "+bakPath+" — restart bot untuk menerapkan perubahan")
}

func cmdMkdir(client *whatsmeow.Client, m *events.Message, args []string) {
	if !isOwner(m) {
		reply(client, m, "❌ Perintah ini khusus owner bot.")
		return
	}
	if len(args) == 0 {
		reply(client, m, "❌ Gunakan: .mkdir path/to/folder")
		return
	}
	p := args[0]
	clean, err := sanitizeRelPath(p)
	if err != nil {
		reply(client, m, "❌ " + err.Error())
		return
	}
	if err := os.MkdirAll(clean, 0o755); err != nil {
		reply(client, m, "❌ Gagal membuat folder: "+err.Error())
		return
	}
	reply(client, m, "✅ Folder dibuat: "+clean)
}

func cmdMkFile(client *whatsmeow.Client, m *events.Message, args []string) {
	if !isOwner(m) {
		reply(client, m, "❌ Perintah ini khusus owner bot.")
		return
	}
	if len(args) == 0 {
		reply(client, m, "❌ Gunakan: .mkfile path/to/file.ext (balas pesan yang berisi isi file)")
		return
	}
	p := args[0]
	clean, err := sanitizeRelPath(p)
	if err != nil {
		reply(client, m, "❌ " + err.Error())
		return
	}
	content, err := requireReplyText(m)
	if err != nil {
		reply(client, m, "❌ " + err.Error())
		return
	}
	// ensure parent dirs
	dir := filepath.Dir(clean)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			reply(client, m, "❌ Gagal membuat folder: "+err.Error())
			return
		}
	}
	if err := os.WriteFile(clean, []byte(content), 0644); err != nil {
		reply(client, m, "❌ Gagal menulis file: "+err.Error())
		return
	}
	reply(client, m, "✅ File dibuat: "+clean)
}
