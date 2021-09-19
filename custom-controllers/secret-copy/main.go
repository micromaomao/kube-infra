package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	yaml "gopkg.in/yaml.v3"
	v1__ "k8s.io/api/core/v1"
	v1___ "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"
	v1_ "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/informers"
	k "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const API_GROUP = "maowtm.org/kube-secret-copy"

func MakeKubeClient(kubeconfig *string) *k.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalln("Unable to build kube config: ", err)
	}
	return k.NewForConfigOrDie(cfg)
}

func loadConfig(configFilePath string) (config *Config) {
	configData, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Fatalf("Unable to read config file: %s\n", err.Error())
		return
	}
	config = &Config{}
	err = yaml.Unmarshal(configData, config)
	if err != nil {
		log.Fatalf("Invalid config file: %s\n", err.Error())
		return
	}
	if config.Namespace == "" {
		log.Fatalln("namespace required in config.")
		return
	}
	if config.CopyMap == nil {
		log.Fatalln("copyMap required in config.")
		return
	}
	return
}

type Config struct {
	Namespace string
	CopyMap   *map[string]string `yaml:"copyMap"`
}

func main() {
	configFile := flag.String("config", "", "Path to config yaml")
	kubeconfig := flag.String("kubeconfig", "", "path to ~/.kube/config, if running outside cluster.")
	flag.Parse()
	if *configFile == "" {
		log.Fatalln("Required -config value")
	}
	config := loadConfig(*configFile)
	client := MakeKubeClient(kubeconfig)
	ctx := context.Background()
	sourceSecretInformer := informers.NewSharedInformerFactoryWithOptions(client, time.Minute, informers.WithNamespace(config.Namespace)).Core().V1().Secrets().Informer()
	deleteFunc := func(name string, targetNamespace string) {
		err := client.CoreV1().Secrets(targetNamespace).Delete(ctx, name, v1___.DeleteOptions{})
		if err != nil {
			panic(fmt.Errorf("Unable to delete %s/%s: %w", targetNamespace, name, err))
		}
		fmt.Printf("Deleted %s/%s\n", targetNamespace, name)
	}
	copyFunc := func(oldObj *v1__.Secret, newObj *v1__.Secret) {
		if newObj == nil {
			if targetNamespace, ok := (*config.CopyMap)[oldObj.Name]; ok {
				deleteFunc(oldObj.Name, targetNamespace)
			}
			return
		}
		if oldObj != nil && oldObj.Name != newObj.Name {
			panic("Name change not possible")
		}
		name := newObj.Name
		targetNamespace, ok := (*config.CopyMap)[name]
		if !ok {
			return
		}
		labels := make(map[string]string)
		labels[API_GROUP] = "true"
		_, err := client.CoreV1().Secrets(targetNamespace).Apply(ctx, &v1.SecretApplyConfiguration{
			TypeMetaApplyConfiguration: v1_.TypeMetaApplyConfiguration{
				Kind:       &[]string{"Secret"}[0],
				APIVersion: &[]string{"v1"}[0],
			},
			ObjectMetaApplyConfiguration: &v1_.ObjectMetaApplyConfiguration{
				Name:        &name,
				Labels: labels,
			},
			Data: newObj.Data,
			Type: &newObj.Type,
		}, v1___.ApplyOptions{
			FieldManager: API_GROUP,
		})
		if err != nil {
			panic(fmt.Errorf("Unable to apply %s: %w", name, err))
		}
		log.Printf("Synced %s -> %s\n", name, targetNamespace)

	}
	sourceSecretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			copyFunc(nil, obj.(*v1__.Secret))
		},
		UpdateFunc: func(old interface{}, newobj interface{}) {
			if old == nil {
				copyFunc(nil, newobj.(*v1__.Secret))
			} else {
				copyFunc(old.(*v1__.Secret), newobj.(*v1__.Secret))
			}
		},
		DeleteFunc: func(obj interface{}) {
			if dfs, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				copyFunc(dfs.Obj.(*v1__.Secret), nil)
			} else {
				copyFunc(obj.(*v1__.Secret), nil)
			}
		},
	})
	sourceSecretInformer.Run(ctx.Done())
}
