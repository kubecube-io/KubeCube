package options

// Options here for avoid import cycle, remove it as soon as we found better way.
type Options struct {
	KubernetesConfig  string
	AllowPrivileged   bool
	LeaderElect       bool
	WebhookCert       string
	WebhookServerPort int
	// ScoutWaitTimeoutSeconds that heartbeat not receive timeout
	ScoutWaitTimeoutSeconds int
	// ScoutInitialDelaySeconds the time that wait for warden start
	ScoutInitialDelaySeconds int
}
