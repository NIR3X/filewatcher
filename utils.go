package filewatcher

func mapDup[K comparable, V any](m map[K]V) map[K]V {
	mCopy := make(map[K]V, len(m))
	for k, v := range m {
		mCopy[k] = v
	}
	return mCopy
}
