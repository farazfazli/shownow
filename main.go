// http://askubuntu.com/a/50000 was a great help to cover edge cases
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"os"
	"strconv"
	"strings"
)

// If an error exists, print message and panic
func checkError(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func main() {
	whoamiFlag := flag.String("whoami", "faraz", "Current Username") // user.Current doesn't work when cross-compiling macOS -> Linux
	userFlag := flag.String("user", "", "Username")
	keyFlag := flag.String("key", "", "Public Key")
	tldFlag := flag.String("tld", "ml", "TLD (ml/cf)")
	tunnelOfflineMessageFlag := flag.String("tunneloffline", "Tunnel offline", "Tunnel offline message to show on a 502 error.")

	flag.Parse()

	whoami := *whoamiFlag
	user := *userFlag
	key := *keyFlag
	tld := *tldFlag
	tunnelOfflineMessage := *tunnelOfflineMessageFlag

	// Validate user & key flags
	if (len(user) < 2 || len(key) < 10) {
		fmt.Println("Please pass in all required arguments")
		return
	}

	ip := getIP()
	port := getFreePort()

	users, err := exec.Command("users").Output()
	checkError("Error getting users", err)

	// Check if user exists
	if strings.Contains(user, "root") || strings.Contains(string(users), user) {
		fmt.Println("User already exists")
		return
	}

	// Add user
	_, err = exec.Command("/bin/sh", "-c", "sudo /usr/sbin/adduser --disabled-password --gecos \"\" "+user).Output()
	checkError("Error creating user", err)
	fmt.Printf("Added user: %s\n", user)

	// Modify .ssh externally
	_, err = exec.Command("/bin/sh", "-c", "cd /home/"+user+" && sudo mkdir .ssh && sudo chown -R "+user+":"+user+" .ssh "+" && cd .ssh && touch authorized_keys && sudo chown "+whoami+":"+whoami+" authorized_keys").Output()
	checkError("Error modifying .ssh", err)
	fmt.Println("Created .ssh skeleton")

	// Add SSH key & prevent user from running other commands
	reversecommand := `ssh ` + user + `@` + ip + ` -N -R ` + port + `:localhost:` + port
	command := `cd /home/` + user + `/.ssh/` + ` && sudo echo "command=\"SHELL=/bin/false && printf 'You cannot login. To tunnel, use the following:\n` + reversecommand + `\n'\",no-agent-forwarding,no-X11-forwarding,permitopen=\"localhost:` + port + `\" ` + key + `" > ` + `authorized_keys && sudo chown ` + user + ":" + user + ` authorized_keys`

	_, err = exec.Command("/bin/sh", "-c", command).Output()
	checkError("Error restricting .ssh to tunnel only", err)
	fmt.Printf("Restricted %s tunneling ability for port %s only\n", user, port)

	subdomain := user + ".shownow." + tld

	nginxConfig, err := os.Create("/etc/nginx/sites-enabled/"+user+".conf")
	checkError("Error creating NGINX configuration", err)
	nginxConfig.WriteString("server { listen 80; server_name "+user+".shownow.ml www."+user+".shownow.ml; location / { proxy_pass http://localhost:"+port+"; proxy_set_header Host $host; proxy_set_header X-Real-IP $remote_addr; proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; proxy_set_header X-Forwarded-Proto $scheme; } error_page 502 /502.html; location = /502.html { return 502 '"+tunnelOfflineMessage+"'; add_header Content-Type text/plain; } }")
	nginxConfig.Sync()

	err = exec.Command("/bin/sh", "-c", "sudo systemctl reload nginx").Start()
	checkError("Error restarting NGINX", err)

	// TODO CloudFlare through API (instead of wildcard) for HTTPS
	fmt.Printf("alias shownow=\"%s && open %s\"\n", reversecommand, subdomain)
}

// https://api.ipify.org - better than myexternalip
func getIP() string {
	resp, err := http.Get("https://api.ipify.org")
	checkError("Error getting IP", err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body)
}

// https://github.com/phayes/freeport/blob/master/freeport.go
func getFreePort() string {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	checkError("resolve", err)

	l, err := net.ListenTCP("tcp", addr)
	checkError("listen", err)

	defer l.Close()
	free := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	fmt.Println("Port: " + free)
	if len(free) < 4 {
		panic("Error getting port")
	}
	return free
}
