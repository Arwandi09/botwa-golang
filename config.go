package main

const (
    PairingNumber = "6285161098098"
)

// Multiple prefix yang didukung
var Prefixes = []string{".",". ",".  ", ":", "!", "?", "/", "*", ",", "\"", "'", "&", "#", ">"}

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