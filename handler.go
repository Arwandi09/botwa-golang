package main

import (
	"botwa/log"
	"botwa/plugin"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

func Handler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch m := evt.(type) {
		case *events.Message:
			// 1. Validasi dasar: Pastikan pesan memiliki konten agar bot tidak crash (nil pointer)
			if m.Message == nil {
				return
			}

			// Log pesan masuk
			log.Raw(m, "!")

			// 2. Cek apakah ini pesan revoke (untuk anti-delete)
			if m.Message.GetProtocolMessage() != nil {
				plugin.HandleDeletedMessage(client, m)
				return
			}

			// 3. CEK VIEW ONCE VIA REPLY
// Fungsi ini HANYA aktif kalau pengguna sendiri me-reply pesan media sekali-lihat.
// Kalau terdeteksi, bot langsung kirim ulang mediannya ke nomor tujuan,
// lalu 'return' supaya tidak diproses lagi oleh cache media biasa di bawah.
if err := plugin.RevealViewOnce(client, m); err != nil {
	return
}


			// 4. Proses Media biasa (Bukan ViewOnce)
			mediaPath := plugin.DownloadAndCacheMedia(client, m)

			// Simpan pesan biasa ke cache untuk kebutuhan Anti-Delete
			plugin.CacheMessage(m, mediaPath)

			// 5. Ambil text dari pesan untuk mengecek Command
			text := m.Message.GetConversation()
			if text == "" && m.Message.GetExtendedTextMessage() != nil {
				text = m.Message.GetExtendedTextMessage().GetText()
			}

			// Cek apakah ada prefix command
			hasPrefix, usedPrefix := hasPrefix(text)
			if hasPrefix {
				// Hapus prefix dari text
				cleanText := usedPrefix + removePrefix(text)
				plugin.Execute(client, m, cleanText)
			}
		}
	}
}
