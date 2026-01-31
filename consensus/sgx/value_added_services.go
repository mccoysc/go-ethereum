package sgx

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ValueAddedServiceManager 增值服务管理器
type ValueAddedServiceManager struct {
	mu       sync.RWMutex
	services map[string]*ValueAddedService
}

// NewValueAddedServiceManager 创建增值服务管理器
func NewValueAddedServiceManager() *ValueAddedServiceManager {
	return &ValueAddedServiceManager{
		services: make(map[string]*ValueAddedService),
	}
}

// RegisterService 注册增值服务
func (vasm *ValueAddedServiceManager) RegisterService(service *ValueAddedService) error {
	vasm.mu.Lock()
	defer vasm.mu.Unlock()

	service.LastUpdateTime = time.Now()
	vasm.services[service.ServiceID] = service

	return nil
}

// GetService 获取服务
func (vasm *ValueAddedServiceManager) GetService(serviceID string) *ValueAddedService {
	vasm.mu.RLock()
	defer vasm.mu.RUnlock()

	service, exists := vasm.services[serviceID]
	if !exists {
		return nil
	}

	serviceCopy := *service
	return &serviceCopy
}

// GetProviderServices 获取提供商的所有服务
func (vasm *ValueAddedServiceManager) GetProviderServices(provider common.Address) []*ValueAddedService {
	vasm.mu.RLock()
	defer vasm.mu.RUnlock()

	services := make([]*ValueAddedService, 0)
	for _, service := range vasm.services {
		if service.Provider == provider {
			serviceCopy := *service
			services = append(services, &serviceCopy)
		}
	}

	return services
}

// EnableService 启用服务
func (vasm *ValueAddedServiceManager) EnableService(serviceID string) error {
	vasm.mu.Lock()
	defer vasm.mu.Unlock()

	service, exists := vasm.services[serviceID]
	if !exists {
		return ErrNoRewardData
	}

	service.Enabled = true
	service.LastUpdateTime = time.Now()

	return nil
}

// DisableService 禁用服务
func (vasm *ValueAddedServiceManager) DisableService(serviceID string) error {
	vasm.mu.Lock()
	defer vasm.mu.Unlock()

	service, exists := vasm.services[serviceID]
	if !exists {
		return ErrNoRewardData
	}

	service.Enabled = false
	service.LastUpdateTime = time.Now()

	return nil
}
