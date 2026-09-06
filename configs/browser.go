package configs

// The service always creates its own browser and isolated context.
type BrowserConfig struct {
	Headless, Stealth, NoSandbox bool
	BinPath, Proxy, UserAgent    string
}
