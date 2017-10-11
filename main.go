package main

import (
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"
)

func getConfig(user, key, password *string) (*ssh.ClientConfig, error) {
	auth := []ssh.AuthMethod{ssh.Password(*password)}
	if len(*key) != 0 {
		keyBytes, err := ioutil.ReadFile(*key)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	return &ssh.ClientConfig{
		User:            *user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}, nil
}
func unuse(arg ...interface{}) {}

func main() {
	flagset := flag.NewFlagSet("ssh/scp", flag.ExitOnError)
	host := flagset.String("host", "localhost:22", "host:port")
	user := flagset.String("user", "root", "user name to login")
	key := flagset.String("key", "", "path to key file")
	password := flagset.String("password", "", "password of user")
	src := flagset.String("src", "", "src path file")
	cmd := flagset.String("cmd", "pwd", "cmd to run")
	shell := flagset.String("shell", "", "shell script file")
	if len(os.Args) < 2 {
		flagset.Usage()
		panic("Wrong Usage")
	}
	unuse(host, user, key, password, src, cmd, shell)

	err := flagset.Parse(os.Args[2:])
	if err != nil {
		panic("error parse flagset " + err.Error())
	}

	log.Printf("%+v, %+v, %+v\n", *host, *user, *password)

	config, err := getConfig(user, key, password)
	if err != nil {
		panic("error get config " + err.Error())
	}

	client, err := ssh.Dial("tcp", *host, config)
	if err != nil {
		panic("failed to dial: " + err.Error())
	}

	switch os.Args[1] {
	case "scp":
		err := copyFile(client, *src)
		if err != nil {
			log.Fatal("error copy file: " + err.Error())
		}
	case "ssh":
		var res string
		if len(*shell) == 0 {
			res, err = exeCmd(client, *cmd)
		} else {
			res, err = exeScript(client, *shell)
		}
		if err != nil {
			log.Fatal("exe failed: " + err.Error())
		}
		fmt.Println(res)
	}
}

func exeCmd(client *ssh.Client, cmd string) (string, error) {
	log.Println(">" + cmd)
	session, err := client.NewSession()
	if err != nil {
		panic("Failed to create session: " + err.Error())
	}
	defer session.Close()
	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(cmd); err != nil {
		log.Println(err.Error())
		return "", err
	}
	return b.String(), nil
}

func exeScript(client *ssh.Client, shell string) (string, error) {
	copyFile(client, shell)
	return exeCmd(client, "/bin/bash ss-tmp/"+path.Base(shell))
}

func copyFile(client *ssh.Client, src string) error {
	log.Println("copy file: " + src)
	session, err := client.NewSession()
	if err != nil {
		panic("Failed to create session: " + err.Error())
	}
	defer session.Close()

	file, err := os.Open(src)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer file.Close()
	info, err := os.Stat(src)
	if err != nil {
		log.Fatal(err.Error())
	}
	go func() {
		w, err := session.StdinPipe()
		if err != nil {
			log.Fatal(err.Error())
		}
		defer w.Close()
		fmt.Fprintln(w, "D0755", 0, "ss-tmp") // mkdir
		fmt.Fprintln(w, "C0755", info.Size(), path.Base(src))
		io.Copy(w, file)
		fmt.Fprint(w, "\x00") // transfer end with \x00
	}()
	if err := session.Run("/usr/bin/scp -tr ./"); err != nil {
		panic("Failed to run: " + err.Error())
	}
	return nil
}
