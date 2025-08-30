package k8s

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"gopkg.in/yaml.v2"

	"github.com/modelcontextprotocol/registry/deploy/infra/pkg/providers"
)

// DeployMonitoringStack deploys a monitoring stack using Victoriametrics, vmagent, and Grafana
func DeployMonitoringStack(ctx *pulumi.Context, cluster *providers.ProviderInfo, environment string) error {
	conf := config.New(ctx, "mcp-registry")
	provider := conf.Get("provider")
	if provider == "" {
		provider = "local"
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
	err = deployVictoriaMetrics(ctx, cluster, monitoringNamespace, environment)
	if err != nil {
		return err
	}

	// deploy vmagent for scraping metrics from registry containers
	err = deployVMAgent(ctx, cluster, monitoringNamespace, environment)
	if err != nil {
		return err
	}

	// deploy grafana for dashboard and alerts
	err = deployGrafana(ctx, cluster.Provider, monitoringNamespace)
	if err != nil {
		return err
	}

	return nil
}

func deployVictoriaMetrics(ctx *pulumi.Context, cluster *providers.ProviderInfo, namespace *corev1.Namespace, environment string) error {
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
				"retentionPeriod": pulumi.String("1d"),
				"extraArgs": pulumi.Map{
					"dedup.minScrapeInterval": pulumi.String("10s"),
					"maxLabelsPerTimeseries":  pulumi.String("50"),
				},
				"resources": pulumi.Map{
					"requests": pulumi.Map{
						"memory": pulumi.String("512Mi"),
						"cpu":    pulumi.String("200m"),
					},
					"limits": pulumi.Map{
						"memory": pulumi.String("1Gi"),
						"cpu":    pulumi.String("500m"),
					},
				},
				"persistentVolume": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
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

func deployVMAgent(ctx *pulumi.Context, cluster *providers.ProviderInfo, namespace *corev1.Namespace, environment string) error {
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
					"scrape_interval": pulumi.String("30s"),
				},
				"scrape_configs": pulumi.Array{
					pulumi.Map{
						"job_name": pulumi.String("kubernetes-pods-annotation"),
						"kubernetes_sd_configs": pulumi.Array{
							pulumi.Map{
								"role": pulumi.String("pod"),
								"namespaces": pulumi.Map{
									"names": pulumi.Array{
										pulumi.String("mcp-registry"),
									},
								},
							},
						},
					},
				},
			},
			"resources": pulumi.Map{
				"requests": pulumi.Map{
					"memory": pulumi.String("256Mi"),
					"cpu":    pulumi.String("100m"),
				},
				"limits": pulumi.Map{
					"memory": pulumi.String("512Mi"),
					"cpu":    pulumi.String("300m"),
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

func deployGrafana(ctx *pulumi.Context, provider *kubernetes.Provider, ns *corev1.Namespace) error {
	cfg := config.New(ctx, "mcp-registry")
	adminPassword := cfg.Get("grafana-admin-password")
	if adminPassword == "" {
		adminPassword = "admin123"
	}

	dbName := cfg.Get("database-name")
	if dbName == "" {
		dbName = "grafana"
	}

	dbUser := cfg.Get("database-user")
	if dbUser == "" {
		dbUser = "grafana"
	}

	dbSSLMode := cfg.Get("database-ssl-mode")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

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

	databaseConfig := pulumi.Map{
		"type": pulumi.String("sqlite3"),
		"path": pulumi.String("/var/lib/grafana/grafana.db"),
	}

	pvcSize := "8Gi"
	grafanaValues := pulumi.Map{
		"replicas": pulumi.Int(1),

		"adminUser":     pulumi.String("admin"),
		"adminPassword": pulumi.String(adminPassword),

		"persistence": pulumi.Map{
			"type":             pulumi.String("pvc"),
			"enabled":          pulumi.Bool(true),
			"storageClassName": pulumi.String(""),
			"size":             pulumi.String(pvcSize),
			"accessModes":      pulumi.Array{pulumi.String("ReadWriteOnce")},
		},

		"resources": pulumi.Map{
			"limits": pulumi.Map{
				"cpu":    pulumi.String("500m"),
				"memory": pulumi.String("512Mi"),
			},
			"requests": pulumi.Map{
				"cpu":    pulumi.String("250m"),
				"memory": pulumi.String("256Mi"),
			},
		},

		// Disable sidecar to avoid mountPath conflicts
		"sidecar": pulumi.Map{
			"dashboards": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
			"datasources": pulumi.Map{
				"enabled": pulumi.Bool(false), // Disabled to avoid conflicts
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
				"enabled": pulumi.Bool(true), // Enable only unified alerting
			},
			"feature_toggles": pulumi.Map{
				"enable": pulumi.String("ngalert"),
			},
			"smtp": pulumi.Map{
				"enabled":      pulumi.Bool(false),
				"host":         pulumi.String("localhost:587"),
				"user":         pulumi.String(""),
				"password":     pulumi.String(""),
				"skip_verify":  pulumi.Bool(false),
				"from_address": pulumi.String("admin@grafana.localhost"),
				"from_name":    pulumi.String("Grafana"),
			},
			"security": pulumi.Map{
				"admin_user":       pulumi.String("admin"),
				"secret_key":       pulumi.String("SW2YcwTIb9zpOOhoPsMm"),
				"disable_gravatar": pulumi.Bool(true),
			},
			"session": pulumi.Map{
				"provider": pulumi.String("file"),
			},
		},

		// Non-conflicting ConfigMap mounts with unique paths
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

		"ingress": pulumi.Map{
			"enabled": pulumi.Bool(false), // Disabled to avoid ingress issues
		},
	}

	// Deploy Grafana with corrected configuration
	_, err = helm.NewChart(ctx, "grafana", helm.ChartArgs{
		Chart:   pulumi.String("grafana"),
		Version: pulumi.String("7.0.19"),
		FetchArgs: &helm.FetchArgs{
			Repo: pulumi.String("https://grafana.github.io/helm-charts"),
		},
		Namespace: ns.Metadata.Name().Elem(),
		Values:    grafanaValues,
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{ns}))
	if err != nil {
		return err
	}

	return nil
}
