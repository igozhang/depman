package hostfile

import (
	"depman/internal/store"
)

// SyncFromStore loads host resolves and applies them to the pod hosts file.
func SyncFromStore(st *store.Store) error {
	if st == nil {
		return nil
	}
	items, err := st.ListHostResolves()
	if err != nil {
		return err
	}
	entries := make([]Entry, 0, len(items))
	for _, it := range items {
		entries = append(entries, Entry{IP: it.IP, Hostname: it.Hostname})
	}
	return Apply(Path(), entries)
}
