package plugin

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "go.mau.fi/whatsmeow"
    waProto "go.mau.fi/whatsmeow/binary/proto"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
    "go.mau.fi/whatsmeow/types/events"
    waLog "go.mau.fi/whatsmeow/util/log"

    _ "github.com/mattn/go-sqlite3"
)

// ============================================================
// JADIBOT — numpang jadi bot dengan sesi WhatsApp sendiri (multi-session)
// Diadaptasi dari plugins/jadibot.js (hukubot1 / Baileys) ke whatsmeow.
//
// Bedanya dengan versi Node:
//   - Reconnect otomatis saat koneksi putus karena jaringan sudah
//     ditangani langsung oleh whatsmeow secara internal, jadi tidak perlu
//     loop reconnect manual seperti di Baileys.
//   - StopJadibotSession() cukup panggil client.Disconnect(), yang memang
//     didesain whatsmeow sebagai "putus sengaja" (tidak memicu reconnect
//     otomatis & tidak mengirim request logout ke WhatsApp).
// ============================================================

const jadibotSessionDir = "session_jadibot"

var (
    jadibotMu sync.Mutex
    JadibotConns = map[string]*whatsmeow.Client{}
)

// isOwner mengecek apakah pengirim pesan adalah owner bot.
// Pakai TargetNumber yang sudah ada di antidelete.go supaya tidak perlu
// konstanta nomor owner baru — ganti nilainya di sana kalau owner beda
// dari nomor yang dipakai untuk notifikasi anti-delete.
func isOwner(m *events.Message) bool {
    return m.Info.Sender.User == TargetNumber
}

func digitsOnly(s string) string {
    var b strings.Builder
    for _, r := range s {
        if r >= '0' && r <= '9' {
            b.WriteRune(r)
        }
    }
    return b.String()
}

func sendText(client *whatsmeow.Client, chat types.JID, text string) {
    client.SendMessage(context.Background(), chat, &waProto.Message{
        Conversation: StringPtr(text),
    })
}

// jadibotPrefixes/jadibotHasPrefix/jadibotRemovePrefix meniru Prefixes &
// helper di config.go (package main). Diduplikasi di sini (bukan di-import)
// supaya package plugin tidak perlu import package main — itu akan bikin
// import cycle karena main.go sudah import "botwa/plugin".
var jadibotPrefixes = []string{".", ":", "!", "?", "/", "*", ",", "\"", "'", "&", "#", ">"}

func jadibotHasPrefix(text string) (bool, string) {
    for _, p := range jadibotPrefixes {
        if len(text) > len(p) && text[:len(p)] == p {
            return true, p
        }
    }
    return false, ""
}

func jadibotRemovePrefix(text string) string {
    for _, p := range jadibotPrefixes {
        if len(text) > len(p) && text[:len(p)] == p {
            return text[len(p):]
        }
    }
    return text
}

// jadibotMessageHandler meniru Handler() di handler.go (package main), supaya
// semua plugin yang sudah ada (menu, ping, brat, dst) otomatis ikut jalan
// juga di akun-akun clone jadibot, bukan cuma di bot utama.
func jadibotMessageHandler(client *whatsmeow.Client) func(interface{}) {
    return func(evt interface{}) {
        switch m := evt.(type) {
        case *events.Message:
            if m.Message == nil {
                return
            }

            if m.Message.GetProtocolMessage() != nil {
                HandleDeletedMessage(client, m)
                return
            }

            if AntiViewOncePasif(client, m) {
                return
            }

            mediaPath := DownloadAndCacheMedia(client, m)
            CacheMessage(m, mediaPath)

            text := m.Message.GetConversation()
            if text == "" && m.Message.GetExtendedTextMessage() != nil {
                text = m.Message.GetExtendedTextMessage().GetText()
            }

            if ok, usedPrefix := jadibotHasPrefix(text); ok {
                cleanText := usedPrefix + jadibotRemovePrefix(text)
                Execute(client, m, cleanText)
            }
        }
    }
}

// StartJadibotSession membuat (atau menyambung ulang) satu sesi clone bot
// untuk targetNumber. notifyClient/notifyChat dipakai untuk mengirim status
// pairing/koneksi — boleh nil kalau tidak butuh notifikasi.
func StartJadibotSession(targetNumber string, notifyClient *whatsmeow.Client, notifyChat types.JID) {
    jadibotMu.Lock()
    if _, exists := JadibotConns[targetNumber]; exists {
        jadibotMu.Unlock()
        return
    }
    jadibotMu.Unlock()

    ctx := context.Background()
    sessionPath := filepath.Join(jadibotSessionDir, targetNumber)
    if err := os.MkdirAll(sessionPath, 0o755); err != nil {
        if notifyClient != nil {
            sendText(notifyClient, notifyChat, "❌ Gagal membuat folder sesi jadibot: "+err.Error())
        }
        return
    }

    dbPath := "file:" + filepath.Join(sessionPath, "session.db") + "?_foreign_keys=on"
    store, err := sqlstore.New(ctx, "sqlite3", dbPath, waLog.Noop)
    if err != nil {
        if notifyClient != nil {
            sendText(notifyClient, notifyChat, "❌ Gagal membuka penyimpanan sesi: "+err.Error())
        }
        return
    }

    device, err := store.GetFirstDevice(ctx)
    if err != nil {
        if notifyClient != nil {
            sendText(notifyClient, notifyChat, "❌ Gagal memuat device sesi: "+err.Error())
        }
        return
    }

    client := whatsmeow.NewClient(device, waLog.Noop)
    client.AddEventHandler(jadibotMessageHandler(client))

    qrReady := make(chan struct{})
    client.AddEventHandler(func(evt interface{}) {
        switch evt.(type) {
        case *events.QR:
            select {
            case <-qrReady:
            default:
                close(qrReady)
            }

        case *events.Connected:
            jadibotMu.Lock()
            JadibotConns[targetNumber] = client
            jadibotMu.Unlock()
            if notifyClient != nil {
                sendText(notifyClient, notifyChat, "✅ Berhasil! Akun "+targetNumber+" sekarang aktif mendampingi bot utama.")
            }

        case *events.LoggedOut:
            jadibotMu.Lock()
            delete(JadibotConns, targetNumber)
            jadibotMu.Unlock()
            if notifyClient != nil {
                sendText(notifyClient, notifyChat, "🚪 Sesi "+targetNumber+" logout dari HP. Sesi jadibot dihentikan permanen.")
            }
        }
    })

    if err := client.Connect(); err != nil {
        if notifyClient != nil {
            sendText(notifyClient, notifyChat, "❌ Gagal konek sesi jadibot: "+err.Error())
        }
        return
    }

    select {
    case <-qrReady:
    case <-time.After(2 * time.Second):
    }

    // Kode pairing cuma diminta kalau sesi ini benar-benar baru.
    if client.Store.ID == nil {
        code, err := client.PairPhone(ctx, targetNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
        if err != nil {
            if notifyClient != nil {
                sendText(notifyClient, notifyChat, "❌ Gagal meminta kode pairing: "+err.Error())
            }
            return
        }

        if notifyClient != nil {
            infoText := "╔══════════════════╗\n" +
                "║  🤖 JADIBOT PAIRING  ║\n" +
                "╚══════════════════╝\n\n" +
                "👤 Target Nomor: " + targetNumber + "\n" +
                "🔑 Kode Pairing: *" + code + "*\n\n" +
                "Silakan buka WhatsApp → Perangkat Tertaut, lalu masukkan kode di atas."
            sendText(notifyClient, notifyChat, infoText)
        }
    }
}

// StopJadibotSession menghentikan satu sesi jadibot yang sedang berjalan.
func StopJadibotSession(targetNumber string) bool {
    jadibotMu.Lock()
    client, ok := JadibotConns[targetNumber]
    if ok {
        delete(JadibotConns, targetNumber)
    }
    jadibotMu.Unlock()

    if !ok {
        return false
    }

    client.Disconnect()
    return true
}

// ActivateAllJadibot menyambungkan ulang semua sesi yang tersimpan di folder
// session_jadibot/, tanpa menyentuh/menghapus file apa pun.
func ActivateAllJadibot(notifyClient *whatsmeow.Client, notifyChat types.JID) []string {
    entries, err := os.ReadDir(jadibotSessionDir)
    if err != nil {
        return nil
    }

    var activated []string
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        targetNumber := entry.Name()

        jadibotMu.Lock()
        _, active := JadibotConns[targetNumber]
        jadibotMu.Unlock()
        if active {
            continue
        }

        dbFile := filepath.Join(jadibotSessionDir, targetNumber, "session.db")
        if _, err := os.Stat(dbFile); err != nil {
            continue
        }

        StartJadibotSession(targetNumber, notifyClient, notifyChat)
        activated = append(activated, targetNumber)
        time.Sleep(1 * time.Second)
    }

    return activated
}

func init() {
    Register(Plugin{
        Command: "jadibot",
        Desc:    "Numpang jadi bot pakai kode pairing (clone akun WA kamu sendiri)",
        Run: func(client *whatsmeow.Client, m *events.Message, args []string) {
            var targetNumber string
            if len(args) > 0 {
                targetNumber = digitsOnly(args[0])
            } else {
                targetNumber = m.Info.Sender.User
            }

            if len(targetNumber) < 9 {
                reply(client, m, "❌ Nomor tidak valid! Masukkan kode negara, contoh: .jadibot 628xxxxxxxxxx")
                return
            }

            jadibotMu.Lock()
            _, active := JadibotConns[targetNumber]
            jadibotMu.Unlock()
            if active {
                reply(client, m, "⚠️ Sesi untuk "+targetNumber+" sudah aktif!")
                return
            }

            reply(client, m, "⏳ Menginisialisasi sesi untuk "+targetNumber+"...")

            go StartJadibotSession(targetNumber, client, m.Info.Chat)
        },
    })
}