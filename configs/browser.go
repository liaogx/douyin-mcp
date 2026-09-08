package configs

// The service always creates its own browser and isolated context.
type BrowserConfig struct {
	Headless, Background, Stealth, NoSandbox bool
	BinPath, Proxy, UserAgent                string
	// Service-owned persistent directory; empty uses a temporary incognito test session.
	ProfileDir string
}
