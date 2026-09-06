package main

const (
    PairingNumber = "6285161098098"

    // Custom pairing code (mirip fitur "code" di neoxr-bot).
    // HARUS persis 8 karakter (huruf/angka), tanpa strip.
    // Kosongkan ("") untuk balik ke kode random bawaan WhatsApp.
    // Dipakai juga oleh plugin jadibot (lihat plugin/jadibot.go).
    PairingCode = "12345678"
)

// Bisa lebih dari satu nomor owner, tinggal tambah di sini
var OwnerNumbers = []string{
    "6285161098098",
    "6282195236600", // tambah nomor owner lain di sini
}

// Multiple prefix yang didukung
var Prefixes = []string{".", ":", "!", "?", "/", "*", ",", "\"", "'", "&", "#", ">"}

// Helper function untuk cek apakah text dimulai dengan salah satu prefix
func hasPrefix(text string) (bool, string) {
    for _, prefix := range Prefixes {
        if len(text) > len(prefix) && text[:len(prefix)] == prefix {
            return true, prefix
        }
    }
    return false, ""
}

// Helper function untuk remove prefix dari text
func removePrefix(text string) string {
    for _, prefix := range Prefixes {
        if len(text) > len(prefix) && text[:len(prefix)] == prefix {
            return text[len(prefix):]
        }
    }
    return text
}