package skill

type Frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

var (
	frontmatterSeparator    = []byte("---\n")
	frontmatterSeparatorWin = []byte("---\r\n")
)
