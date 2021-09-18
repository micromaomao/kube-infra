package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	yaml "gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apply "k8s.io/client-go/applyconfigurations/core/v1"
	applymeta "k8s.io/client-go/applyconfigurations/meta/v1"
	k "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func MakeKubeClient(kubeconfig *string) *k.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalln("Unable to build kube config: ", err)
	}
	return k.NewForConfigOrDie(cfg)
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
	var configData []byte
	var config Config
	loadConfig := func() (err error, changed bool) {
		newConfigData, err := ioutil.ReadFile(*configFile)
		if err != nil {
			err = errors.New("Unable to read config file: " + err.Error())
			return
		}
		if configData == nil || !bytes.Equal(configData, newConfigData) {
			log.Println("Reloaded config.")
			configData = newConfigData
			changed = true
		} else {
			return
		}
		config = Config{}
		err = yaml.Unmarshal(configData, &config)
		if err != nil {
			err = errors.New("Invalid config file: " + err.Error())
			return
		}
		if config.Namespace == "" {
			err = errors.New("namespace required in config.")
			return
		}
		if config.CopyMap == nil {
			err = errors.New("copyMap required in config.")
			return
		}
		return
	}
	err, _ := loadConfig()
	if err != nil {
		log.Fatalln("Error reading config: ", err)
	}
	client := MakeKubeClient(kubeconfig)
	ctx := context.Background()
	watch, err := client.CoreV1().Secrets(config.Namespace).Watch(ctx, v1.ListOptions{})
	if err != nil {
		log.Fatalln("Failed to watch secrets: ", err)
	}
	watchChan := watch.ResultChan()
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	configWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Warning: unable to create file watcher. Will not reload config.")
		configWatcher = nil
	} else {
		configWatcher.Add(*configFile)
		defer configWatcher.Close()
	}
	update := func() {
		copyMap := *config.CopyMap
		res, err := client.CoreV1().Secrets(config.Namespace).List(ctx, v1.ListOptions{})
		if err != nil {
			log.Fatalln("Unable to list secrets: ", err.Error())
		}
		unprocessed := make(map[string]string)
		for n, t := range copyMap {
			unprocessed[n] = t
		}
		for _, r := range res.Items {
			name := r.Name
			targetNamespace, exists := copyMap[name]
			if !exists {
				continue
			}
			delete(unprocessed, name)
			_, err = client.CoreV1().Secrets(targetNamespace).Apply(ctx, &apply.SecretApplyConfiguration{
				TypeMetaApplyConfiguration: applymeta.TypeMetaApplyConfiguration{
					Kind:       &[]string{"Secret"}[0],
					APIVersion: &[]string{"v1"}[0],
				},
				ObjectMetaApplyConfiguration: &applymeta.ObjectMetaApplyConfiguration{
					Name: &r.Name,
				},
				Data: r.Data,
			}, v1.ApplyOptions{
				FieldManager: "maowtm.org/kube-secret-copy",
			})
			if err != nil {
				panic(errors.New(fmt.Sprintf("Unable to apply %s: %s", r.Name, err.Error())))
			}
			log.Printf("Synced %s -> %s\n", r.Name, targetNamespace)
		}
		for name, targetNamespace := range unprocessed {
			err := client.CoreV1().Secrets(targetNamespace).Delete(ctx, name, v1.DeleteOptions{})
			if err != nil && !strings.Contains(err.Error(), "not found") {
				log.Fatalf("Unable to delete %s in %s: %s\n", name, targetNamespace, err.Error())
			}
			if err == nil {
				log.Printf("Deleted %s (%s)\n", name, targetNamespace)
			} else {
				log.Printf("%s does not exist in either %s or %s\n", name, config.Namespace, targetNamespace)
			}
		}
		log.Println("Sync finished.")
	}
	update()
	configWatcherChan := make(chan fsnotify.Event)
	if configWatcher != nil {
		configWatcherChan = configWatcher.Events
	}
	for {
		select {
		case <-watchChan:
			update()
		case <-timer.C:
			update()
		case <-configWatcherChan:
			err, changed := loadConfig()
			if err != nil {
				log.Printf("Warning: failed to reload config file: %s\n", err.Error())
			} else if changed {
				update()
			}
		}
	}
}
