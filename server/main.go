package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/websocket"
	"github.com/iden3/go-circuits"
	auth "github.com/iden3/go-iden3-auth"
	"github.com/iden3/go-iden3-auth/loaders"
	"github.com/iden3/go-iden3-auth/pubsignals"
	"github.com/iden3/go-iden3-auth/state"
	"github.com/iden3/iden3comm/protocol"
	"github.com/rs/cors"
	uuid "github.com/satori/go.uuid"
)

type Server struct {
	Config Configuration
}

func NewServer(cfg Configuration) *Server {
	return &Server{
		Config: cfg,
	}
}

type SocketMessage struct {
	Fn     FunctionName  `json:"fn,omitempty"`
	Status RequestStatus `json:"status,omitempty"`
	Data   interface{}   `json:"data,omitempty"`
}

func main() {
	var err error
	cfg, err := Load()
	if err != nil {
		log.Panic(context.Background(), "cannot load config", "err", err)
		return
	}

	server := NewServer(*cfg)

	mux := http.NewServeMux()

	mux.HandleFunc(API_PATH_GET_AUTH_QR, server.GetAuthQr)
	mux.HandleFunc(API_PATH_VERIFICATION_CALLBACK, server.HandleVerification)
	mux.HandleFunc(WEBSOCKET_PATH_GET_SESSION_ID, server.GetSessionId)
	corsOptions := cors.New(cors.Options{
		AllowedOrigins: []string{cfg.FrontendUrl},
	})
	handler := corsOptions.Handler(mux)
	http.ListenAndServe(":8080", handler)
}

// Create a map to store the auth requests and their session IDs
var requestMap = make(map[string]interface{})

// Create a map to store the websocket connections and their session IDs
var wsConnections = make(map[string]*websocket.Conn)

func (s *Server) GetAuthQr(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")

	// fetch websocket connection from sessionID
	wsConn, wsConnExists := wsConnections[sessionID]

	if wsConnExists {
		// Send socket message
		err := sendSocketMessage(wsConn, SocketMessage{Fn: GET_AUTH_QR, Status: IN_PROGRESS, Data: sessionID})
		if err != nil {
			log.Println(err)
		}
	}

	rURL := s.Config.HostedServerUrl

	uri := fmt.Sprintf("%s%s?sessionId=%s", rURL, API_PATH_VERIFICATION_CALLBACK, sessionID)

	// Generate request for basic authentication
	var request protocol.AuthorizationRequestMessage = auth.CreateAuthorizationRequest("Must be born before this year", s.Config.VerifierDID, uri)

	request.ID = sessionID
	request.ThreadID = sessionID

	// Add request for a specific proof
	var mtpProofRequest protocol.ZeroKnowledgeProofRequest
	mtpProofRequest.ID = 1
	mtpProofRequest.CircuitID = string(circuits.AtomicQuerySigV2CircuitID)
	mtpProofRequest.Query = map[string]interface{}{
		"allowedIssuers": []string{"*"},
		"credentialSubject": map[string]interface{}{
			"birthday": map[string]interface{}{
				"$lt": 20230101,
			},
		},
		"context": "https://raw.githubusercontent.com/iden3/claim-schema-vocab/main/schemas/json-ld/kyc-v3.json-ld",
		"type":    "KYCAgeCredential",
	}
	request.Body.Scope = append(request.Body.Scope, mtpProofRequest)

	// Store auth request in map associated with session ID
	requestMap[sessionID] = request

	if wsConnExists {
		// Send socket message
		err := sendSocketMessage(wsConn, SocketMessage{Fn: GET_AUTH_QR, Status: DONE, Data: request})
		if err != nil {
			log.Println(err)
		}
	}

	// print request
	fmt.Println(request)

	msgBytes, _ := json.Marshal(request)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(msgBytes)
}

// HandleVerification works with /api/get-auth-qr callbacks
func (s *Server) HandleVerification(w http.ResponseWriter, r *http.Request) {

	// Get session ID from request
	sessionID := r.URL.Query().Get("sessionId")

	// fetch authRequest from sessionID
	authRequest := requestMap[sessionID]

	// print authRequest
	fmt.Println(authRequest)

	// fetch websocket connection from sessionID
	wsConn, wsConnExists := wsConnections[sessionID]

	if wsConnExists {
		// Send socket message
		err := sendSocketMessage(wsConn, SocketMessage{Fn: HANDLE_VERIFICATION, Status: IN_PROGRESS, Data: sessionID})
		if err != nil {
			log.Println(err)
		}
	}

	// get JWZ token params from the post request
	tokenBytes, _ := io.ReadAll(r.Body)

	// Add Polygon Mumbai RPC node endpoint - needed to read on-chain state
	ethURL := s.Config.RpcUrlMumbai

	// Add identity state contract address
	contractAddress := "0x134B1BE34911E39A8397ec6289782989729807a4"

	resolverPrefix := "polygon:mumbai"

	// Locate the directory that contains circuit's verification keys
	keyDIR := "./keys"

	// load the verifcation key
	var verificationKeyloader = &loaders.FSKeyLoader{Dir: keyDIR}
	resolver := state.ETHResolver{
		RPCUrl:          ethURL,
		ContractAddress: common.HexToAddress(contractAddress),
	}

	resolvers := map[string]pubsignals.StateResolver{
		resolverPrefix: resolver,
	}

	// EXECUTE VERIFICATION
	verifier := auth.NewVerifier(verificationKeyloader, loaders.DefaultSchemaLoader{IpfsURL: "ipfs.io"}, resolvers)
	authResponse, err := verifier.FullVerify(
		r.Context(),
		string(tokenBytes),
		authRequest.(protocol.AuthorizationRequestMessage),
		pubsignals.WithAcceptedStateTransitionDelay(time.Minute*5))
	if err != nil {
		log.Println(err.Error())
		if wsConnExists {
			// Send socket message
			err := sendSocketMessage(wsConn, SocketMessage{Fn: HANDLE_VERIFICATION, Status: ERROR, Data: err})
			if err != nil {
				log.Println(err)
			}
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userID := authResponse.From
	if wsConnExists {
		// Send socket message
		err := sendSocketMessage(wsConn, SocketMessage{Fn: HANDLE_VERIFICATION, Status: DONE, Data: authResponse})
		if err != nil {
			log.Println(err)
		}
	}

	messageBytes := []byte("User with ID " + userID + " Successfully authenticated")

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(messageBytes)
}

func (s *Server) GetSessionId(w http.ResponseWriter, r *http.Request) {
	// We'll need to define an Upgrader
	// this will require a Read and Write buffer size
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// Check origin
	upgrader.CheckOrigin = func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == s.Config.FrontendUrl
	}

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Generate session ID
	sessionId := uuid.NewV4().String()
	wsConnections[sessionId] = ws

	log.Println("Client Connected")
	err = ws.WriteMessage(1, []byte(`{"sessionId" : "`+sessionId+`"}`))
	if err != nil {
		log.Println(err)
		return
	}
}

func sendSocketMessage(wsConn *websocket.Conn, socketMessage SocketMessage) error {
	message, err := json.Marshal(socketMessage)
	if err != nil {
		return err
	}
	err = wsConn.WriteMessage(1, []byte(message))
	if err != nil {
		return err
	}
	return nil
}
