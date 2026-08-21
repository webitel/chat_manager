package facebook

import (
	"fmt"
	"strings"

	"github.com/micro/micro/v3/service/errors"
)

var RequireBatchAPIVersion = MustPareGraphAPIVersion("v26.0")

type APIVersion struct {
	Major int
	Minor int
}

func ParseAPIVersion(s string) (APIVersion, error) {
	s = strings.TrimPrefix(s, "v")
	var v APIVersion

	if _, err := fmt.Sscanf(s, "%d.%d", &v.Major, &v.Minor); err != nil {
		return APIVersion{}, errors.BadRequest(
			"facebook.graph_version.parse_api_version.scaning",
			"scanning input version into structure: %s",
			s,
		)
	}

	return v, nil
}

func MustPareGraphAPIVersion(s string) APIVersion {
	v, err := ParseAPIVersion(s)
	if err != nil {
		panic(err)
	}

	return v
}

func (v *APIVersion) Compare(other *APIVersion) int {
	if v.Major != other.Major {
		if v.Major > other.Major {
			return 1
		}
		return -1
	}
	if v.Minor != other.Minor {
		if v.Minor > other.Minor {
			return 1
		}
		return -1
	}
	return 0
}

func (v *APIVersion) GreaterOrEqual(other *APIVersion) bool { return v.Compare(other) >= 0 }

func (v *APIVersion) String() string {
	return fmt.Sprintf("v%d.%d", v.Major, v.Minor)
}
