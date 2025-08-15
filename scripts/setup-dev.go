package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	fmt.Println("🚀 Setting up Payment Gateway Development Environment")
	
	// Check Docker
	if err := checkDocker(); err != nil {
		fmt.Printf("⚠️  Docker issue detected: %v\n", err)
		fmt.Println("💡 You can still run in mock mode: make dev-mock")
		return
	}
	
	fmt.Println("✅ Docker is running")
	fmt.Println("🐳 Starting Kafka services...")
	
	cmd := exec.Command("docker-compose", "up", "-d", "kafka", "redis")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to start services: %v\n", err)
		fmt.Println("💡 Try: make dev-mock")
		return
	}
	
	fmt.Println("✅ Services started successfully!")
	fmt.Println("🎯 Run: make dev")
}

func checkDocker() error {
	cmd := exec.Command("docker", "info")
	return cmd.Run()
}
