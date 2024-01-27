package filewatcher

type filePath struct {
	Value string
	Depth int
}

// Paths is a slice of Path
type filePathList []filePath

// Len, Less, and Swap are needed to implement the sort.Interface
func (p filePathList) Len() int      { return len(p) }
func (p filePathList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p filePathList) Less(i, j int) bool {
	// Sort by depth in descending order
	return p[i].Depth > p[j].Depth
}
