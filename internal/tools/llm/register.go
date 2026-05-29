package llm

import (
	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/llm"
)

// RegisterTools registra todas las herramientas del paquete llm en el registry.
func RegisterTools(reg *executor.Registry, manager *llm.Manager, providersConf *config.ProvidersConfig, configPath string) error {
	if err := reg.Register(NewListProvidersTool(manager)); err != nil {
		return err
	}
	if err := reg.Register(NewGetStatusTool(manager)); err != nil {
		return err
	}
	if err := reg.Register(NewUseProviderTool(manager)); err != nil {
		return err
	}
	if err := reg.Register(NewSwitchModelTool(manager)); err != nil {
		return err
	}
	if err := reg.Register(NewListModelsTool(manager)); err != nil {
		return err
	}
	if err := reg.Register(NewListProfilesTool(manager, providersConf)); err != nil {
		return err
	}
	if err := reg.Register(NewUseProfileTool(manager, providersConf)); err != nil {
		return err
	}
	if err := reg.Register(NewCreateProfileTool(manager, providersConf, configPath)); err != nil {
		return err
	}
	if err := reg.Register(NewVerifyProviderTool(manager)); err != nil {
		return err
	}
	return nil
}
