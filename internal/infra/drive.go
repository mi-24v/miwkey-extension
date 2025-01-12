package infra

type MisskeyDriveClientInterface interface {
	Delete(id string) error
}

type MisskeyDriveClient struct {
	token string
}
