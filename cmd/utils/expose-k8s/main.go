package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Service struct {
	Name       string
	Namespace  string
	LocalPort  string
	RemotePort string
}

func runPortForward(s Service, kubeconfig string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		args := []string{}
		if kubeconfig != "" {
			args = append(args, "--kubeconfig", kubeconfig)
		}
		args = append(args, "port-forward", "-n", s.Namespace, "svc/"+s.Name, s.LocalPort+":"+s.RemotePort)
		cmd := exec.Command("kubectl", args...)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			fmt.Printf("[%s] Не удалось запустить: %v\n", s.Name, err)
			return
		}
		var once sync.Once
		stopCh := make(chan struct{})
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				fmt.Printf("[%s][stdout] %s\n", s.Name, scanner.Text())
			}
			once.Do(func() { close(stopCh) })
		}()
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				fmt.Printf("[%s][stderr] %s\n", s.Name, scanner.Text())
			}
			once.Do(func() { close(stopCh) })
		}()
		cmd.Wait()
		fmt.Printf("[%s] port-forward завершён, переподключение через 2 секунды...\n", s.Name)
		<-stopCh
		time.Sleep(2 * time.Second)
	}
}

func parseServiceFlag(s string) (Service, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 4 {
		return Service{}, fmt.Errorf("неверный формат флага svc: %s (ожидался формат name:namespace:localPort:remotePort)", s)
	}
	return Service{
		Name:       parts[0],
		Namespace:  parts[1],
		LocalPort:  parts[2],
		RemotePort: parts[3],
	}, nil
}

func main() {
	var svcFlags multiFlag
	var kubeconfig string
	flag.Var(&svcFlags, "svc", "Сервис для проброса портов в формате name:namespace:localPort:remotePort (можно использовать несколько раз)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Путь к kubeconfig для kubectl")
	flag.Parse()

	if len(svcFlags) == 0 {
		fmt.Println("Нужно указать сервисы для проброса портов! Формат: --svc name:namespace:localPort:remotePort (можно использовать несколько раз)")
		os.Exit(1)
	}

	services := []Service{}
	for _, s := range svcFlags {
		service, err := parseServiceFlag(s)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		services = append(services, service)
	}

	var wg sync.WaitGroup
	for _, s := range services {
		wg.Add(1)
		go runPortForward(s, kubeconfig, &wg)
	}
	wg.Wait()
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
