//go:build !windows

package launcher

func registerPlatformConsoleCloseBridge(func()) (func(), error) {
	return nil, nil
}
