package plugin

import (
    "sync"
    "time"

    "go.mau.fi/whatsmeow/types/events"
)

type CachedMessage struct {
    Message   *events.Message
    Timestamp time.Time
    MediaPath string // path file jika ada media
}

var (
    messageCache = make(map[string]*CachedMessage)
    cacheMutex   sync.RWMutex
)

// Simpan pesan ke cache
func CacheMessage(m *events.Message, mediaPath string) {
    cacheMutex.Lock()
    defer cacheMutex.Unlock()

    key := m.Info.ID
    messageCache[key] = &CachedMessage{
        Message:   m,
        Timestamp: time.Now(),
        MediaPath: mediaPath,
    }

    // Auto cleanup cache yang lebih dari 24 jam
    go cleanupOldCache()
}

// Ambil pesan dari cache
func GetCachedMessage(messageID string) *CachedMessage {
    cacheMutex.RLock()
    defer cacheMutex.RUnlock()

    return messageCache[messageID]
}

// Hapus pesan dari cache
func RemoveCachedMessage(messageID string) {
    cacheMutex.Lock()
    defer cacheMutex.Unlock()

    delete(messageCache, messageID)
}

// Cleanup cache lama (lebih dari 24 jam)
func cleanupOldCache() {
    cacheMutex.Lock()
    defer cacheMutex.Unlock()

    now := time.Now()
    for key, cached := range messageCache {
        if now.Sub(cached.Timestamp) > 24*time.Hour {
            delete(messageCache, key)
        }
    }
}