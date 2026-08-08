package fetchver

// ResolveNode resolves a Node.js version query and returns the exact version.
// This is the programmatic entry point used by globver.
func ResolveNode(query string) (string, error) {
	return resolveNode(query)
}

// ResolveNpmVersion resolves an npm package version query.
func ResolveNpmVersion(pkg, query string) (string, error) {
	result, err := resolveNpm(pkg, query)
	if err != nil {
		return "", err
	}
	// Strip the package prefix: "typescript@5.9.3" → "5.9.3"
	if i := lastIndex(result, "@"); i >= 0 {
		return result[i+1:], nil
	}
	return result, nil
}

// ResolveGopkgVersion resolves a Go package version query.
func ResolveGopkgVersion(pkg, query string) (string, error) {
	result, err := resolveGopkg(pkg, query)
	if err != nil {
		return "", err
	}
	if i := lastIndex(result, "@"); i >= 0 {
		return result[i+1:], nil
	}
	return result, nil
}

func lastIndex(s, sep string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep[0] {
			return i
		}
	}
	return -1
}
