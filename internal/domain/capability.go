package domain

// CapabilityStatus 厂商能力支持状态
type CapabilityStatus string

const (
	CapVerified    CapabilityStatus = "verified"
	CapUnsupported CapabilityStatus = "unsupported"
	CapUnverified  CapabilityStatus = "unverified"
)

// ProviderCapabilities 厂商适配器支持的能力集清单
type ProviderCapabilities struct {
	Discover CapabilityStatus `json:"discover"`
	Start    CapabilityStatus `json:"start"`
	Stop     CapabilityStatus `json:"stop"`
	Restart  CapabilityStatus `json:"restart"`
	Create   CapabilityStatus `json:"create"`
	Clone    CapabilityStatus `json:"clone"`
	Reset    CapabilityStatus `json:"reset"`
	Delete   CapabilityStatus `json:"delete"`
	Headless CapabilityStatus `json:"headless"`
	Metrics  CapabilityStatus `json:"metrics"`
}
