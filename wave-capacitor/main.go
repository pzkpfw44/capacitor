// main.go - Wave Capacitor with DHT integration
package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	
	"wave-capacitor/config"
	"wave-capacitor/dht"
	"wave-capacitor/models"
	"wave-capacitor/routes"
	
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Starting Wave Capacitor...
	log.Println("🔹 Starting Wave Capacitor with DHT support")

	// Load configuration
	config.LoadConfig()
	
	// Load DHT configuration
	dhtConfig := config.LoadDHTConfig()
	
	// Create the DHT storage directory
	if err := dhtConfig.MakeDHTStorageDirectory(); err != nil {
		log.Fatalf("❌ Failed to create DHT storage directory: %v", err)
	}
	
	// Initialize the database before starting Fiber
	if err := models.InitializeDB(); err != nil {
		log.Fatalf("❌ Database initialization failed: %v", err)
	}
	log.Println("✅ Database initialized")
	
	// Initialize DHT
	dht, err := initializeDHT(dhtConfig)
	if err != nil {
		log.Fatalf("❌ DHT initialization failed: %v", err)
	}
	log.Printf("✅ DHT initialized with node ID: %s", dht.LocalNode().ID.String())
	
	// Create a new Fiber instance
	app := fiber.New(fiber.Config{
		AppName: "Wave Capacitor v1.0",
	})

	// Add middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))
	app.Use(logger.New())

	// Root endpoint for API info
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Wave Capacitor - Making waves in the universe, one message at a time.",
			"version": "1.0",
			"node_id": dht.LocalNode().ID.String(),
			"node_type": "capacitor",
			"endpoints": []string{
				"/api/register",
				"/api/login",
				"/api/recover_account",
				"/api/logout",
				"/api/get_public_key",
				"/api/get_encrypted_private_key",
				"/api/send_message",
				"/api/get_messages",
				"/api/add_contact",
				"/api/get_contacts",
				"/api/remove_contact",
				"/api/backup_account",
				"/api/delete_account",
				"/dht/status", // New DHT status endpoint
			},
			"status": "Online",
		})
	})

	// Add DHT status endpoint
	app.Get("/dht/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"node_id": dht.LocalNode().ID.String(),
			"routing_table_size": dht.RoutingTableSize(),
			"known_peers": dht.KnownPeers(),
			"node_type": "capacitor",
			"bootstrap_nodes": dhtConfig.BootstrapNodes,
		})
	})

	// Add DHT ping endpoint to test connectivity to other nodes
	app.Get("/dht/ping", func(c *fiber.Ctx) error {
		address := c.Query("address")
		if address == "" {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error": "Missing address parameter",
			})
		}
		
		// Ping the node
		success, nodeInfo, err := dht.PingNode(address)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error": err.Error(),
			})
		}
		
		return c.JSON(fiber.Map{
			"success": success,
			"node_info": nodeInfo,
		})
	})

	// Add DHT findservice endpoint
	app.Get("/dht/findservice", func(c *fiber.Ctx) error {
		serviceType := c.Query("type", "locker") // Default to finding locker services
		
		services, err := dht.FindServicesByType(serviceType)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error": err.Error(),
			})
		}
		
		return c.JSON(fiber.Map{
			"success": true,
			"services": services,
		})
	})

	// Setup API routes
	routes.SetupRoutes(app)

	// Create required directories for message and contact storage
	config.EnsureDirectoriesExist()

	// Register this service in the DHT
	registerCapacitorService(dht, dhtConfig)
	
	// Start the DHT
	if err := dht.Start(); err != nil {
		log.Fatalf("❌ Failed to start DHT: %v", err)
	}
	log.Println("✅ DHT service started")
	
	// Create a channel to listen for shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	// Start the server in a goroutine
	go func() {
		// Start the server
		port := config.GetPort()
		log.Printf("🚀 Wave Capacitor running on http://localhost:%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()
	
	// Block until we receive a shutdown signal
	<-quit
	log.Println("🛑 Shutting down server...")
	
	// Create a timeout context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Stop the DHT
	if err := dht.Stop(); err != nil {
		log.Printf("⚠️ Error stopping DHT: %v", err)
	}
	
	// Shutdown the server
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatalf("❌ Server shutdown failed: %v", err)
	}
	
	log.Println("👋 Server gracefully stopped")
}

// initializeDHT initializes the DHT service for the capacitor
func initializeDHT(cfg *config.DHTConfig) (*dht.DHT, error) {
	// Create DHT configuration
	dhtCfg := &dht.DHTConfig{
		BootstrapNodes:  cfg.BootstrapNodes,
		ListenAddr:      cfg.GetDHTAddress(),
		APIPort:         cfg.APIPort,
		GRPCPort:        cfg.GRPCPort,
		RefreshInterval: cfg.RefreshInterval,
		NodeType:        "capacitor", // Explicitly set as capacitor
		NumShards:       cfg.NumShards,
		StoreDir:        cfg.StoragePath,
	}
	
	// Create DHT instance
	return dht.NewDHT(dhtCfg)
}

// registerCapacitorService registers this capacitor as a service in the DHT
func registerCapacitorService(d *dht.DHT, cfg *config.DHTConfig) {
	// Create a unique service ID based on node ID
	serviceID := "capacitor:" + d.LocalNode().ID.String()
	
	// Get the external IP
	externalIP := cfg.ExternalIP
	if externalIP == "" {
		// In a production environment, you should implement a proper
		// external IP detection mechanism
		externalIP = getOutboundIP().String()
	}
	
	// Create service info
	info := dht.ServiceInfo{
		NodeID:     d.LocalNode().ID,
		NodeType:   "capacitor",
		Address:    externalIP + ":" + strconv.Itoa(cfg.APIPort),
		APIPort:    cfg.APIPort,
		GRPCPort:   cfg.GRPCPort,
		NumShards:  cfg.NumShards,
		Version:    "1.0.0",
		Properties: map[string]string{
			"environment": os.Getenv("ENVIRONMENT"),
			"role": "message_processor",
		},
		LastSeen:   time.Now(),
	}
	
	// Register the service
	if err := d.RegisterService(serviceID, info); err != nil {
		log.Printf("⚠️ Failed to register service: %v", err)
	} else {
		log.Println("✅ Capacitor service registered in DHT")
	}
}

// getOutboundIP gets the preferred outbound IP of this machine
func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("⚠️ Failed to determine outbound IP: %v", err)
		return net.ParseIP("127.0.0.1")
	}
	defer conn.Close()
	
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}