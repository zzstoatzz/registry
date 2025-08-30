package k8s

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"gopkg.in/yaml.v2"

	"github.com/modelcontextprotocol/registry/deploy/infra/pkg/providers"
)

// EnvironmentConfig holds environment-specific configuration
type EnvironmentConfig struct {
	Provider              string
	IsProduction          bool
	VictoriaMetricsConfig VictoriaMetricsConfig
	GrafanaConfig         GrafanaConfig
	VMAgentConfig         VMAgentConfig
}

type VictoriaMetricsConfig struct {
	RetentionPeriod   string
	PersistentEnabled bool
	StorageSize       string
	CPURequest        string
	CPULimit          string
	MemoryRequest     string
	MemoryLimit       string
	ReplicaCount      int
}

type GrafanaConfig struct {
	DatabaseType      string
	DatabaseHost      string
	DatabaseName      string
	DatabaseUser      string
	AdminPassword     string
	DatabaseSSLMode   string
	PersistentEnabled bool
	StorageSize       string
	CPURequest        string
	CPULimit          string
	MemoryRequest     string
	MemoryLimit       string
}

type VMAgentConfig struct {
	CPURequest     string
	CPULimit       string
	MemoryRequest  string
	MemoryLimit    string
	ScrapeInterval string
}

// getEnvironmentConfig returns environment-specific configuration
func getEnvironmentConfig(ctx *pulumi.Context) *EnvironmentConfig {
	conf := config.New(ctx, "mcp-registry")
	provider := conf.Get("provider")
	if provider == "" {
		provider = "local"
	}

	isProduction := provider != "local"

	if isProduction {
		return &EnvironmentConfig{
			Provider:     provider,
			IsProduction: true,
			VictoriaMetricsConfig: VictoriaMetricsConfig{
				RetentionPeriod:   conf.Get("victoria-metrics-retention-period"),
				PersistentEnabled: true,
				StorageSize:       conf.Get("victoria-metrics-storage-size"),
				CPURequest:        "250m",
				CPULimit:          "500m",
				MemoryRequest:     "512Mi",
				MemoryLimit:       "2Gi",
				ReplicaCount:      1,
			},
			GrafanaConfig: GrafanaConfig{
				DatabaseType:      conf.Get("grafana-database-type"),
				DatabaseHost:      conf.Get("grafana-database-host"),
				DatabaseName:      conf.Get("grafana-database-name"),
				DatabaseUser:      conf.Get("grafana-database-user"),
				AdminPassword:     conf.Get("grafana-admin-password"),
				DatabaseSSLMode:   conf.Get("grafana-database-ssl-mode"),
				PersistentEnabled: true,
				StorageSize:       conf.Get("grafana-storage-size"),
				CPURequest:        "250m",
				CPULimit:          "500m",
				MemoryRequest:     "256Mi",
				MemoryLimit:       "512Mi",
			},
			VMAgentConfig: VMAgentConfig{
				CPURequest:     "250m",
				CPULimit:       "500m",
				MemoryRequest:  "256Mi",
				MemoryLimit:    "512Mi",
				ScrapeInterval: "30s",
			},
		}
	} else {
		// Local development configuration
		return &EnvironmentConfig{
			Provider:     provider,
			IsProduction: false,
			VictoriaMetricsConfig: VictoriaMetricsConfig{
				RetentionPeriod:   "1d",
				PersistentEnabled: conf.GetBool("victoria-metrics-persistent-enabled"), // Allow override via config
				StorageSize:       "2Gi",
				CPURequest:        "200m",
				CPULimit:          "300m",
				MemoryRequest:     "256Mi",
				MemoryLimit:       "512Gi",
				ReplicaCount:      1,
			},
			GrafanaConfig: GrafanaConfig{
				DatabaseType:      "sqlite3",
				DatabaseHost:      "",
				DatabaseName:      "",
				DatabaseUser:      "",
				AdminPassword:     "admin123", // Default for local dev
				DatabaseSSLMode:   "",
				PersistentEnabled: conf.GetBool("grafana-persistent-enabled"), // Allow override via config
				StorageSize:       "1Gi",
				CPURequest:        "250m",
				CPULimit:          "500m",
				MemoryRequest:     "256Mi",
				MemoryLimit:       "512Mi",
			},
			VMAgentConfig: VMAgentConfig{
				CPURequest:     "100m",
				CPULimit:       "300m",
				MemoryRequest:  "256Mi",
				MemoryLimit:    "512Mi",
				ScrapeInterval: "30s",
			},
		}
	}
}

// setDefaultProductionConfigs sets default values if not provided in config
func setDefaultProductionConfigs(envConfig *EnvironmentConfig, conf *config.Config) {
	if envConfig.VictoriaMetricsConfig.RetentionPeriod == "" {
		envConfig.VictoriaMetricsConfig.RetentionPeriod = "15d"
	}
	if envConfig.VictoriaMetricsConfig.StorageSize == "" {
		envConfig.VictoriaMetricsConfig.StorageSize = "10Gi"
	}

	// Grafana defaults - Use CNPG PostgreSQL for all non-local environments
	if envConfig.GrafanaConfig.DatabaseType == "" {
		envConfig.GrafanaConfig.DatabaseType = "postgres"
	}
	if envConfig.GrafanaConfig.DatabaseHost == "" {
		envConfig.GrafanaConfig.DatabaseHost = "grafana-pg-rw:5432" // CNPG read-write service
	}
	if envConfig.GrafanaConfig.DatabaseName == "" {
		envConfig.GrafanaConfig.DatabaseName = "grafana"
	}
	if envConfig.GrafanaConfig.DatabaseUser == "" {
		envConfig.GrafanaConfig.DatabaseUser = "app"
	}
	if envConfig.GrafanaConfig.AdminPassword == "" {
		envConfig.GrafanaConfig.AdminPassword = "admin123" // Should be overridden via config
	}
	if envConfig.GrafanaConfig.DatabaseSSLMode == "" {
		envConfig.GrafanaConfig.DatabaseSSLMode = "disable" // CNPG internal connections can use disable
	}
	if envConfig.GrafanaConfig.StorageSize == "" {
		envConfig.GrafanaConfig.StorageSize = "5Gi"
	}
}

// DeployMonitoringStack deploys a monitoring stack using Victoriametrics, vmagent, and Grafana
func DeployMonitoringStack(ctx *pulumi.Context, cluster *providers.ProviderInfo, environment string) error {
	envConfig := getEnvironmentConfig(ctx)

	if envConfig.IsProduction {
		conf := config.New(ctx, "mcp-registry")
		setDefaultProductionConfigs(envConfig, conf)
	}

	// Create namespace for monitoring
	monitoringNamespace, err := corev1.NewNamespace(ctx, "monitoring", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("monitoring"),
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// deploy victoriametrics for storing metrics data
	err = deployVictoriaMetrics(ctx, cluster, monitoringNamespace, environment, envConfig)
	if err != nil {
		return err
	}

	// deploy vmagent for scraping metrics from registry containers
	err = deployVMAgent(ctx, cluster, monitoringNamespace, environment, envConfig)
	if err != nil {
		return err
	}

	// Deploy PostgreSQL for non-local environments
	var pgCluster *apiextensions.CustomResource
	if envConfig.IsProduction {
		pgCluster, err = deployGrafanaDatabase(ctx, cluster, environment, envConfig)
		if err != nil {
			return err
		}
	}

	// deploy grafana for dashboard and alerts
	err = deployGrafana(ctx, cluster.Provider, monitoringNamespace, envConfig, pgCluster)
	if err != nil {
		return err
	}

	return nil
}

func deployVictoriaMetrics(ctx *pulumi.Context, cluster *providers.ProviderInfo, namespace *corev1.Namespace, environment string, envConfig *EnvironmentConfig) error {
	vmConfig := envConfig.VictoriaMetricsConfig

	// Conditional persistent volume configuration
	var persistentVolumeConfig pulumi.Map
	if vmConfig.PersistentEnabled {
		persistentVolumeConfig = pulumi.Map{
			"enabled":     pulumi.Bool(true),
			"size":        pulumi.String(vmConfig.StorageSize),
			"accessModes": pulumi.Array{pulumi.String("ReadWriteOnce")},
		}
	} else {
		persistentVolumeConfig = pulumi.Map{
			"enabled": pulumi.Bool(false),
		}
	}

	// Install VictoriaMetrics Single using Helm chart
	_, err := helm.NewChart(ctx, "victoria-metrics-single", helm.ChartArgs{
		Chart:     pulumi.String("victoria-metrics-single"),
		Version:   pulumi.String("0.19.0"),
		Namespace: namespace.Metadata.Name().Elem(),
		FetchArgs: helm.FetchArgs{
			Repo: pulumi.String("https://victoriametrics.github.io/helm-charts/"),
		},
		Values: pulumi.Map{
			"server": pulumi.Map{
				"retentionPeriod": pulumi.String(vmConfig.RetentionPeriod),
				"replicaCount":    pulumi.Int(vmConfig.ReplicaCount),
				"extraArgs": pulumi.Map{
					"dedup.minScrapeInterval": pulumi.String("10s"),
					"maxLabelsPerTimeseries":  pulumi.String("50"),
				},
				"resources": pulumi.Map{
					"requests": pulumi.Map{
						"memory": pulumi.String(vmConfig.MemoryRequest),
						"cpu":    pulumi.String(vmConfig.CPURequest),
					},
					"limits": pulumi.Map{
						"memory": pulumi.String(vmConfig.MemoryLimit),
						"cpu":    pulumi.String(vmConfig.CPULimit),
					},
				},
				"persistentVolume": persistentVolumeConfig,
				"service": pulumi.Map{
					"type": pulumi.String("ClusterIP"),
					"port": pulumi.Int(8428),
				},
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	return nil
}

func deployVMAgent(ctx *pulumi.Context, cluster *providers.ProviderInfo, namespace *corev1.Namespace, environment string, envConfig *EnvironmentConfig) error {
	agentConfig := envConfig.VMAgentConfig

	_, err := helm.NewChart(ctx, "victoria-metrics-agent", helm.ChartArgs{
		Chart:     pulumi.String("victoria-metrics-agent"),
		Version:   pulumi.String("0.12.0"),
		Namespace: namespace.Metadata.Name().Elem(),
		FetchArgs: helm.FetchArgs{
			Repo: pulumi.String("https://victoriametrics.github.io/helm-charts/"),
		},
		Values: pulumi.Map{
			"remoteWriteUrls": pulumi.Array{
				pulumi.String("http://victoria-metrics-single-server:8428/api/v1/write"),
			},
			"config": pulumi.Map{
				"global": pulumi.Map{
					"scrape_interval": pulumi.String(agentConfig.ScrapeInterval),
				},
				"scrape_configs": pulumi.Array{
					pulumi.Map{
						"job_name": pulumi.String("mcp-registry-pods"),
						"kubernetes_sd_configs": pulumi.Array{
							pulumi.Map{
								"role": pulumi.String("pod"),
								"namespaces": pulumi.Map{
									"names": pulumi.Array{
										pulumi.String("default"),
									},
								},
							},
						},
						"relabel_configs": pulumi.Array{
							pulumi.Map{
								"source_labels": pulumi.Array{
									pulumi.String("__meta_kubernetes_pod_label_app"),
								},
								"regex":  pulumi.String("mcp-registry.*"),
								"action": pulumi.String("keep"),
							},
						},
					},
				},
			},
			"resources": pulumi.Map{
				"requests": pulumi.Map{
					"memory": pulumi.String(agentConfig.MemoryRequest),
					"cpu":    pulumi.String(agentConfig.CPURequest),
				},
				"limits": pulumi.Map{
					"memory": pulumi.String(agentConfig.MemoryLimit),
					"cpu":    pulumi.String(agentConfig.CPULimit),
				},
			},
			"extraArgs": pulumi.Map{
				"memory.allowedPercent":    pulumi.String("80"),
				"promscrape.maxScrapeSize": pulumi.String("64MB"),
			},
			"rbac": pulumi.Map{
				"create": pulumi.Bool(true),
			},
			"serviceAccount": pulumi.Map{
				"create": pulumi.Bool(true),
				"name":   pulumi.String("vmagent"),
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	return nil
}

// deployGrafanaDatabase deploys the CloudNative PostgreSQL operator and PostgreSQL cluster
func deployGrafanaDatabase(ctx *pulumi.Context, cluster *providers.ProviderInfo, environment string, envConfig *EnvironmentConfig) (*apiextensions.CustomResource, error) {
	// Environment-specific PostgreSQL cluster configuration
	var pgStorageSize string
	var instances int

	pgStorageSize = "1Gi"
	instances = 1
	if envConfig.IsProduction {
		pgStorageSize = "5Gi"
	}

	clusterSpec := map[string]any{
		"instances": instances,
		"storage": map[string]any{
			"size": pgStorageSize,
		},
		"bootstrap": map[string]any{
			"initdb": map[string]any{
				"database": "grafana",
				"owner":    "app",
			},
		},
	}

	// Create PostgreSQL cluster (CNPG will auto-create grafana-pg-app secret)
	pgCluster, err := apiextensions.NewCustomResource(ctx, "grafana-pg", &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("postgresql.cnpg.io/v1"),
		Kind:       pulumi.String("Cluster"),
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("grafana-pg"),
			Namespace: pulumi.String("monitoring"),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("grafana-pg"),
				"environment": pulumi.String(environment),
			},
		},
		OtherFields: map[string]any{
			"spec": clusterSpec,
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return nil, err
	}

	return pgCluster, nil
}

func deployGrafana(ctx *pulumi.Context, provider *kubernetes.Provider, ns *corev1.Namespace, envConfig *EnvironmentConfig, pgCluster *apiextensions.CustomResource) error {
	grafanaConfig := envConfig.GrafanaConfig
	adminPassword := grafanaConfig.AdminPassword
	datasourcesConfig := map[string]interface{}{
		"apiVersion": 1,
		"datasources": []map[string]interface{}{
			{
				"name":      "VictoriaMetrics",
				"type":      "prometheus",
				"url":       "http://victoria-metrics-single-server:8428",
				"access":    "proxy",
				"isDefault": true,
				"uid":       "victoriametrics",
			},
		},
	}

	datasourcesConfigYAML, _ := yaml.Marshal(datasourcesConfig)
	_, err := corev1.NewConfigMap(ctx, "grafana-datasources-config", &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("grafana-datasources-config"),
			Namespace: ns.Metadata.Name(),
		},
		Data: pulumi.StringMap{
			"datasources.yaml": pulumi.String(string(datasourcesConfigYAML)),
		},
	}, pulumi.Provider(provider))
	if err != nil {
		return err
	}

	// Configure database based on environment
	var databaseConfig pulumi.Map
	var grafanaDependsOn []pulumi.Resource

	if envConfig.IsProduction {
		// Use CNPG PostgreSQL - Grafana will get connection via environment variable
		databaseConfig = pulumi.Map{
			"type": pulumi.String(envConfig.GrafanaConfig.DatabaseType),
		}
		if pgCluster != nil {
			grafanaDependsOn = append(grafanaDependsOn, pgCluster)
		}
	} else {
		// Local development uses SQLite
		databaseConfig = pulumi.Map{
			"type": pulumi.String("sqlite3"),
			"path": pulumi.String("/var/lib/grafana/grafana.db"),
		}
	}

	// Conditional persistent volume configuration
	var persistenceConfig pulumi.Map
	if grafanaConfig.PersistentEnabled {
		persistenceConfig = pulumi.Map{
			"type":        pulumi.String("pvc"),
			"enabled":     pulumi.Bool(true),
			"size":        pulumi.String(grafanaConfig.StorageSize),
			"accessModes": pulumi.Array{pulumi.String("ReadWriteOnce")},
		}
	} else {
		persistenceConfig = pulumi.Map{
			"type":    pulumi.String("emptyDir"),
			"enabled": pulumi.Bool(false),
		}
	}

	grafanaValues := pulumi.Map{
		"replicas":      pulumi.Int(1),
		"adminUser":     pulumi.String("admin"),
		"adminPassword": pulumi.String(adminPassword),
		"persistence":   persistenceConfig,
		"resources": pulumi.Map{
			"limits": pulumi.Map{
				"cpu":    pulumi.String(grafanaConfig.CPULimit),
				"memory": pulumi.String(grafanaConfig.MemoryLimit),
			},
			"requests": pulumi.Map{
				"cpu":    pulumi.String(grafanaConfig.CPURequest),
				"memory": pulumi.String(grafanaConfig.MemoryRequest),
			},
		},

		// Disable sidecar to avoid mountPath conflicts
		"sidecar": pulumi.Map{
			"dashboards": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
			"datasources": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
		},
		"grafana.ini": pulumi.Map{
			"server": pulumi.Map{
				"root_url": pulumi.String("%(protocol)s://%(domain)s/"),
			},
			"database": databaseConfig,
			"alerting": pulumi.Map{
				"enabled": pulumi.Bool(false), // Disable legacy alerting
			},
			"unified_alerting": pulumi.Map{
				"enabled": pulumi.Bool(true), // Enable unified alerting
			},
			"feature_toggles": pulumi.Map{
				"enable": pulumi.String("ngalert"), // Modern alerting system
			},
			// SMTP disabled - no email alerts needed
			"smtp": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
			"security": pulumi.Map{
				"admin_user":       pulumi.String("admin"),
				"secret_key":       pulumi.String("SW2YcwTIb9zpOOhoPsMm"),
				"disable_gravatar": pulumi.Bool(true), // Block external requests
			},
			// File sessions for single instance
			"session": pulumi.Map{
				"provider": pulumi.String("file"),
			},
		},
		// ConfigMap mounts
		"extraConfigmapMounts": pulumi.Array{
			pulumi.Map{
				"name":      pulumi.String("grafana-datasources-config"),
				"mountPath": pulumi.String("/etc/grafana/provisioning/datasources"),
				"configMap": pulumi.String("grafana-datasources-config"),
				"readOnly":  pulumi.Bool(true),
			},
		},
		"service": pulumi.Map{
			"type": pulumi.String("ClusterIP"),
			"port": pulumi.Int(80),
		},
		// Ingress disabled - no external access needed
		"ingress": pulumi.Map{
			"enabled": pulumi.Bool(false),
		},
		// RBAC configuration - only for production
		"rbac": pulumi.Map{
			"create": pulumi.Bool(envConfig.IsProduction),
		},
		"serviceAccount": pulumi.Map{
			"create": pulumi.Bool(envConfig.IsProduction),
		},
	}

	// Inject database URL via environment variable in production
	if envConfig.IsProduction {
		grafanaValues["env"] = pulumi.Map{
			"GF_DATABASE_TYPE":     pulumi.String("postgres"),
			"GF_DATABASE_HOST":     pulumi.String("grafana-pg-rw"),
			"GF_DATABASE_PORT":     pulumi.String("5432"),
			"GF_DATABASE_NAME":     pulumi.String("grafana"),
			"GF_DATABASE_SSL_MODE": pulumi.String("require"),
		}

		grafanaValues["envValueFrom"] = pulumi.Map{
			"GF_DATABASE_USER": pulumi.Map{
				"secretKeyRef": pulumi.Map{
					"name": pulumi.String("grafana-pg-app"),
					"key":  pulumi.String("username"),
				},
			},
			"GF_DATABASE_PASSWORD": pulumi.Map{
				"secretKeyRef": pulumi.Map{
					"name": pulumi.String("grafana-pg-app"),
					"key":  pulumi.String("password"),
				},
			},
		}

	}

	// Add namespace to dependencies
	grafanaDependsOn = append(grafanaDependsOn, ns)

	// Deploy Grafana with environment-aware configuration
	_, err = helm.NewChart(ctx, "grafana", helm.ChartArgs{
		Chart:   pulumi.String("grafana"),
		Version: pulumi.String("7.0.19"),
		FetchArgs: &helm.FetchArgs{
			Repo: pulumi.String("https://grafana.github.io/helm-charts"),
		},
		Namespace: ns.Metadata.Name().Elem(),
		Values:    grafanaValues,
	}, pulumi.Provider(provider), pulumi.DependsOn(grafanaDependsOn))
	if err != nil {
		return err
	}

	return nil
}
