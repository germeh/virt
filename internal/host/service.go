package host

import "context"

type Service struct {
	inspector Inspector
	driver    VMDriver
}

type VMDriver interface {
	ListVMs(ctx context.Context) ([]VM, error)
}

func NewService(inspector Inspector, driver VMDriver) *Service {
	return &Service{inspector: inspector, driver: driver}
}

func NewLocalService() *Service {
	return NewService(LocalInspector{}, VirshDriver{Runner: ExecRunner{}})
}

func (s *Service) Capabilities(ctx context.Context) (Capabilities, error) {
	return DetectCapabilities(ctx, s.inspector), nil
}

func (s *Service) ListVMs(ctx context.Context) ([]VM, error) {
	return s.driver.ListVMs(ctx)
}
