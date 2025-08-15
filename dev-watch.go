//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	fmt.Println("🔥 Payment Gateway Hot Reload Server")
	fmt.Println("📁 Watching for file changes...")
	
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Add directories to watch
	dirs := []string{".", "internal", "internal/handlers", "internal/services", "internal/models", "internal/kafka", "internal/logger", "internal/middleware", "internal/config", "internal/storage"}
	
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			err = watcher.Add(dir)
			if err != nil {
				log.Printf("Error watching %s: %v", dir, err)
			} else {
				fmt.Printf("👀 Watching: %s\n", dir)
			}
		}
	}

	var cmd *exec.Cmd
	restart := make(chan bool, 1)
	
	// Start the application
	go startApp(&cmd, restart)
	
	// Initial start
	restart <- true

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			
			// Only restart on Go file changes
			if strings.HasSuffix(event.Name, ".go") && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				fmt.Printf("🔄 File changed: %s\n", filepath.Base(event.Name))
				fmt.Println("🔨 Rebuilding and restarting...")
				
				// Stop current process
				if cmd != nil && cmd.Process != nil {
					cmd.Process.Kill()
				}
				
				// Restart after a short delay
				time.Sleep(500 * time.Millisecond)
				restart <- true
			}
			
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Error:", err)
		}
	}
}

func startApp(cmd **exec.Cmd, restart <-chan bool) {
	for range restart {
		// Build the application
		fmt.Println("🔨 Building application...")
		buildCmd := exec.Command("go", "build", "-o", "payment-gateway", ".")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		
		if err := buildCmd.Run(); err != nil {
			fmt.Printf("❌ Build failed: %v\n", err)
			continue
		}
		
		fmt.Println("✅ Build successful!")
		fmt.Println("🚀 Starting Payment Gateway...")
		fmt.Println("" + strings.Repeat("=", 50))
		
		// Start the application
		*cmd = exec.Command("./payment-gateway")
		(*cmd).Stdout = os.Stdout
		(*cmd).Stderr = os.Stderr
		
		if err := (*cmd).Start(); err != nil {
			fmt.Printf("❌ Failed to start: %v\n", err)
			continue
		}
		
		// Wait for the process to finish
		go func() {
			(*cmd).Wait()
		}()
	}
}
