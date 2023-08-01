package auth

import "github.com/webitel/chat_manager/api/proto/auth"

var (
	PermissionCreateAny = auth.Permission{
		Id:    `add`,
		Name:  `Create`,
		Usage: `Grants permission to create any objects`,
	}
	PermissionSelectAny = auth.Permission{
		Id:    `read`,
		Name:  `Select`,
		Usage: `Grants permission to select any objects`,
	}
	PermissionUpdateAny = auth.Permission{
		Id:    `write`,
		Name:  `Update`,
		Usage: `Grants permission to modify any objects`,
	}
	PermissionDeleteAny = auth.Permission{
		Id:    `delete`,
		Name:  `Delete`,
		Usage: `Grants permission to remove any objects`,
	}
)
