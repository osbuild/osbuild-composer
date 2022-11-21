/*
Copyright (c) 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// IMPORTANT: This file has been generated automatically, refrain from modifying it manually as all
// your changes will be lost when the file is generated again.

package metrics // github.com/openshift-online/ocm-sdk-go/metrics

// pathTreeData is the JSON representation of the tree of URL paths.
var pathTreeData = `{
  "api": {
    "accounts_mgmt": {
      "v1": {
        "access_token": null,
        "accounts": {
          "-": {
            "labels": {
              "-": null
            }
          }
        },
        "cloud_resources": {
          "-": null
        },
        "cluster_authorizations": null,
        "cluster_registrations": null,
        "current_access": {
          "-": null
        },
        "current_account": null,
        "feature_toggles": {
          "-": {
            "query": null
          }
        },
        "labels": null,
        "notify": null,
        "organizations": {
          "-": {
            "labels": {
              "-": null
            },
            "quota_cost": null,
            "resource_quota": {
              "-": null
            },
            "summary_dashboard": null
          }
        },
        "permissions": {
          "-": null
        },
        "pull_secrets": {
          "-": null
        },
        "registries": {
          "-": null
        },
        "registry_credentials": {
          "-": null
        },
        "resource_quota": {
          "-": null
        },
        "role_bindings": {
          "-": null
        },
        "roles": {
          "-": null
        },
        "sku_rules": {
          "-": null
        },
        "subscriptions": {
          "-": {
            "labels": {
              "-": null
            },
            "notify": null,
            "reserved_resources": {
              "-": null
            },
            "role_bindings": {
              "-": null
            }
          },
          "labels": {
            "-": null
          }
        },
        "support_cases": {
          "-": null
        },
        "token_authorization": null
      }
    },
    "addons_mgmt": {
      "v1": {
        "addons": {
          "-": {
            "versions": {
              "-": null
            }
          }
        }
      }
    },
    "authorizations": {
      "v1": {
        "access_review": null,
        "capability_review": null,
        "export_control_review": null,
        "feature_review": null,
        "resource_review": null,
        "self_access_review": null,
        "self_capability_review": null,
        "self_feature_review": null,
        "self_terms_review": null,
        "terms_review": null
      }
    },
    "clusters_mgmt": {
      "v1": {
        "addons": {
          "-": {
            "versions": {
              "-": null
            }
          }
        },
        "aws_infrastructure_access_roles": {
          "-": null
        },
        "aws_inquiries": {
          "machine_types": null,
          "regions": null,
          "sts_credential_requests": null,
          "sts_policies": null,
          "vpcs": null
        },
        "cloud_providers": {
          "-": {
            "available_regions": null,
            "regions": {
              "-": null
            }
          }
        },
        "clusters": {
          "-": {
            "addon_inquiries": {
              "-": null
            },
            "addons": {
              "-": null
            },
            "aws_infrastructure_access_role_grants": {
              "-": null
            },
            "clusterdeployment": null,
            "credentials": null,
            "external_configuration": {
              "labels": {
                "-": null
              },
              "manifests": {
                "-": null
              },
              "syncsets": {
                "-": null
              }
            },
            "gate_agreements": {
              "-": null
            },
            "groups": {
              "-": {
                "users": {
                  "-": null
                }
              }
            },
            "hibernate": null,
            "hypershift": null,
            "identity_providers": {
              "-": {
                "htpasswd_users": {
                  "-": null,
                  "import": null
                }
              }
            },
            "ingresses": {
              "-": null
            },
            "limited_support_reasons": {
              "-": null
            },
            "logs": {
              "install": null,
              "uninstall": null
            },
            "machine_pools": {
              "-": null
            },
            "metric_queries": {
              "alerts": null,
              "cluster_operators": null,
              "cpu_total_by_node_roles_os": null,
              "nodes": null,
              "socket_total_by_node_roles_os": null
            },
            "product": null,
            "provision_shard": null,
            "resources": {
              "live": null
            },
            "resume": null,
            "status": null,
            "sts_operator_roles": {
              "-": null
            },
            "upgrade_policies": {
              "-": {
                "state": null
              }
            }
          }
        },
        "environment": null,
        "events": null,
        "flavours": {
          "-": null
        },
        "gcp_inquiries": {
          "encryption_keys": null,
          "key_rings": null,
          "regions": null,
          "vpcs": null
        },
        "limited_support_reason_templates": {
          "-": null
        },
        "machine_types": null,
        "products": {
          "-": null
        },
        "provision_shards": {
          "-": null
        },
        "version_gates": {
          "-": null
        },
        "versions": {
          "-": null
        }
      }
    },
    "job_queue": {
      "v1": {
        "queues": {
          "-": {
            "jobs": {
              "-": {
                "failure": null,
                "success": null
              }
            },
            "pop": null,
            "push": null
          }
        }
      }
    },
    "osd_fleet_mgmt": {
      "v1": {
        "management_clusters": {
          "-": null
        },
        "service_clusters": {
          "-": null
        }
      }
    },
    "service_logs": {
      "v1": {
        "cluster_logs": {
          "-": null
        },
        "clusters": {
          "-": {
            "cluster_logs": null
          }
        }
      }
    },
    "service_mgmt": {
      "v1": {
        "services": {
          "-": null
        }
      }
    },
    "status_board": {
      "v1": {
        "application_dependencies": {
          "-": null
        },
        "applications": {
          "-": {
            "services": {
              "-": {
                "statuses": {
                  "-": null
                }
              }
            }
          }
        },
        "errors": {
          "-": null
        },
        "peer_dependencies": {
          "-": null
        },
        "products": {
          "-": {
            "applications": {
              "-": {
                "services": {
                  "-": {
                    "statuses": {
                      "-": null
                    }
                  }
                }
              }
            },
            "updates": {
              "-": null
            }
          }
        },
        "services": {
          "-": {
            "statuses": {
              "-": null
            }
          }
        },
        "status_updates": {
          "-": null
        },
        "statuses": {
          "-": null
        }
      }
    },
    "web_rca": {
      "v1": {
        "errors": {
          "-": null
        },
        "incidents": {
          "-": {
            "events": {
              "-": {
                "attachments": {
                  "-": null
                }
              }
            },
            "follow_ups": {
              "-": null
            },
            "notifications": {
              "-": null
            }
          }
        },
        "users": {
          "-": null
        }
      }
    }
  }
}
`
