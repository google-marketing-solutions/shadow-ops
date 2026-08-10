// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		portFlag        = flag.String("port", "", "Port to listen on")
		workspaceIDFlag = flag.String("workspace-id", "", "Workspace identifier")
		realAgentFlag   = flag.Bool("real", true, "Use real Playwright browser engine")
		headlessFlag    = flag.Bool("headless", false, "Run browser in headless mode")
	)
	flag.Parse()

	port := *portFlag
	if port == "" {
		port = os.Getenv("AGENT_PORT")
	}
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8765"
	}

	workspaceID := *workspaceIDFlag
	if workspaceID == "" {
		workspaceID = os.Getenv("WORKSPACE_ID")
	}
	if workspaceID == "" {
		workspaceID = "ws_local"
	}

	isReal := *realAgentFlag
	if envReal := os.Getenv("REAL_AGENT"); envReal == "false" {
		isReal = false
	}

	isHeadless := *headlessFlag
	if envHeadless := os.Getenv("BROWSER_HEADLESS"); envHeadless == "true" {
		isHeadless = true
	}

	mgr := NewStateManager(workspaceID)

	var engine BrowserEngine
	if isReal {
		log.Println("Initializing RealAgentEngine (Playwright-Go)...")
		opts := RealAgentEngineOptions{
			WorkspaceID: workspaceID,
			CDPURL:      os.Getenv("CHROME_CDP_URL"),
			Headless:    isHeadless,
			ScriptsDir:  "scripts",
		}
		realEng, err := NewRealAgentEngine(mgr, opts)
		if err != nil {
			log.Printf("Warning: failed to initialize RealAgentEngine: %v. Falling back to MockAgentEngine.", err)
			engine = NewMockAgentEngine(mgr)
		} else {
			engine = realEng
		}
	} else {
		log.Println("Initializing MockAgentEngine...")
		engine = NewMockAgentEngine(mgr)
	}
	defer engine.Close()

	srv := NewAgentServer(mgr, engine)
	mux := http.NewServeMux()
	srv.SetupRoutes(mux)
	srv.StartFrameLoop()
	defer srv.StopFrameLoop()

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("[agent] workspace=%s listening on %s (real=%v)", workspaceID, addr, isReal)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("Received termination signal")
	case <-srv.shutdownCh:
		log.Println("Received kill endpoint shutdown request")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	log.Printf("[agent] workspace=%s stopped", workspaceID)
}
