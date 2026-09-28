package updater

import (
	"strconv"
	"strings"
)

// CompareSemver compares two semantic version strings (e.g. "v1.7.1" and "v1.7.2").
// Returns -1 if v1 < v2, 1 if v1 > v2, and 0 if v1 == v2.
func CompareSemver(v1, v2 string) int {
	v1Clean := cleanVersion(v1)
	v2Clean := cleanVersion(v2)

	if v1Clean == v2Clean {
		return 0
	}
	// "dev" is considered older than any published release
	if v1Clean == "dev" && v2Clean != "dev" {
		return -1
	}
	if v2Clean == "dev" && v1Clean != "dev" {
		return 1
	}

	core1, pre1 := splitPreRelease(v1Clean)
	core2, pre2 := splitPreRelease(v2Clean)

	parts1 := parseVersionNumbers(core1)
	parts2 := parseVersionNumbers(core2)

	maxLen := max(len(parts1), len(parts2))

	for i := 0; i < maxLen; i++ {
		num1, num2 := 0, 0
		if i < len(parts1) {
			num1 = parts1[i]
		}
		if i < len(parts2) {
			num2 = parts2[i]
		}
		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	// When core versions are equal:
	// A version without a pre-release has higher precedence than one with a pre-release
	// (e.g. 1.7.1 > 1.7.1-rc1)
	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 == "" {
		return -1
	}
	if pre1 != pre2 {
		if pre1 < pre2 {
			return -1
		}
		return 1
	}

	return 0
}

func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

func splitPreRelease(v string) (string, string) {
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

func parseVersionNumbers(core string) []int {
	segments := strings.Split(core, ".")
	nums := make([]int, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		n, err := strconv.Atoi(seg)
		if err != nil {
			// If contains non-numeric suffix, attempt to strip
			var numStr string
			for _, r := range seg {
				if r >= '0' && r <= '9' {
					numStr += string(r)
				} else {
					break
				}
			}
			if numStr != "" {
				n, _ = strconv.Atoi(numStr)
			}
		}
		nums = append(nums, n)
	}
	return nums
}
