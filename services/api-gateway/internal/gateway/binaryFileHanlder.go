package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure" // TODO: Use secure credentials
	"google.golang.org/grpc/status"

	waterPb "golang-microservices-boilerplate/proto/water-quality-service" // Adjust import path if needed
	"golang-microservices-boilerplate/services/api-gateway/internal/domain"
)

const (
	// uploadTimeout defines the maximum duration for the entire upload process.
	uploadTimeout = 10 * time.Minute
	// maxUploadSize defines the maximum allowed size for multipart form parsing (e.g., 32MB).
	// maxUploadSize = 32 << 20
	// 1GB
	maxUploadSize = 1 << 30
	// chunkSize defines the buffer size for reading file chunks.
	// chunkSize = 64 * 1024 // 64KB
	chunkSize = 1 << 20 // 1MB
)

// grpcStatusToHTTP maps gRPC status codes to HTTP status codes.
func grpcStatusToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout // Or 499 Client Closed Request if using Nginx/specific proxies
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// registerWaterQualityCustomHandlers registers custom handlers specific to the Water Quality service.
// Currently, this only includes the binary file upload handler.
func registerWaterQualityCustomHandlers(mux *runtime.ServeMux, service domain.Service) error {
	// Get the target service address from the discovered service info
	waterQualityServiceAddr := service.Endpoint
	if waterQualityServiceAddr == "" {
		return fmt.Errorf("cannot register custom handlers: endpoint missing for service %s", service.Name)
	}

	uploadPath := "/api/v1/water-quality/upload"

	// Register the custom handler for the specific upload path
	err := mux.HandlePath("POST", uploadPath, handleWaterQualityUpload(waterQualityServiceAddr))
	if err != nil {
		return fmt.Errorf("failed to register custom handler for path %s on service %s: %w", uploadPath, service.Name, err)
	}

	// Log success (optional, could be logged in the calling function)
	// fmt.Printf("Registered custom handler for %s path on service %s\n", uploadPath, service.Name)

	// Add more custom handlers for this service here if needed
	return nil
}

// handleWaterQualityUpload returns the custom HTTP handler function for water quality file uploads.
// This version waits for the gRPC upload to complete before sending the HTTP response.
func handleWaterQualityUpload(waterQualityServiceAddr string) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		// 1. Parse Multipart Form
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, fmt.Sprintf("failed to parse multipart form: %v", err), http.StatusBadRequest)
			return
		}

		filename := r.FormValue("filename")
		if filename == "" {
			http.Error(w, "filename is required", http.StatusBadRequest)
			return
		}

		fileType := r.FormValue("file_type")
		if fileType == "" {
			http.Error(w, "file_type is required", http.StatusBadRequest)
			return
		}

		// 3. Extract File Field
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get form file 'file': %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close() // Ensure file is closed when handler exits

		// --- Start Synchronous Processing ---

		// Use request context with timeout for the entire gRPC operation
		ctx, cancel := context.WithTimeout(r.Context(), uploadTimeout)
		defer cancel()

		// 4. Establish gRPC Client Connection
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // FIXME: Use secure credentials!
		}
		conn, err := grpc.NewClient(waterQualityServiceAddr, opts...)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to connect to water quality service (%s): %v", waterQualityServiceAddr, err), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := waterPb.NewWaterQualityServiceClient(conn)

		// 5. Call Streaming RPC
		stream, err := client.UploadData(ctx)
		if err != nil {
			st, _ := status.FromError(err)
			http.Error(w, fmt.Sprintf("failed to start upload stream: %s", st.Message()), grpcStatusToHTTP(st.Code()))
			return
		}

		// 6. Send Metadata Messages
		if err := stream.Send(&waterPb.UploadRequest{Payload: &waterPb.UploadRequest_Filename{Filename: filename}}); err != nil {
			st, _ := status.FromError(err)
			// Don't try CloseAndRecv here, the stream is likely broken. Report send error.
			http.Error(w, fmt.Sprintf("failed to send filename: %s", st.Message()), grpcStatusToHTTP(st.Code()))
			return
		}
		if err := stream.Send(&waterPb.UploadRequest{Payload: &waterPb.UploadRequest_FileType{FileType: fileType}}); err != nil {
			st, _ := status.FromError(err)
			http.Error(w, fmt.Sprintf("failed to send filetype: %s", st.Message()), grpcStatusToHTTP(st.Code()))
			return
		}

		// 7. Stream File Content
		buffer := make([]byte, chunkSize)
		for {
			n, readErr := file.Read(buffer)
			if n > 0 {
				if sendErr := stream.Send(&waterPb.UploadRequest{Payload: &waterPb.UploadRequest_DataChunk{DataChunk: buffer[:n]}}); sendErr != nil {
					st, _ := status.FromError(sendErr)
					// Check if server closed stream gracefully (might appear as EOF or specific gRPC code)
					// If the error is EOF, it might mean the server processed everything and closed.
					// We will rely on CloseAndRecv to get the final status in this case.
					if sendErr == io.EOF || st.Code() == codes.Canceled || st.Code() == codes.Unavailable {
						fmt.Printf("INFO: Send encountered %v, proceeding to CloseAndRecv\n", sendErr)
						break // Exit loop and attempt CloseAndRecv
					}
					// For other errors, report them immediately.
					http.Error(w, fmt.Sprintf("failed to send data chunk: %s", st.Message()), grpcStatusToHTTP(st.Code()))
					return // Exit handler
				}
			}
			if readErr == io.EOF {
				break // End of file reached
			}
			if readErr != nil {
				// Don't try to CloseSend, just report the internal error reading the file.
				http.Error(w, fmt.Sprintf("error reading file chunk: %v", readErr), http.StatusInternalServerError)
				return
			}
		}

		// 8. Close Stream and Get Response
		resp, err := stream.CloseAndRecv()
		if err != nil {
			// Check specifically for EOF, which might indicate the server closed
			// the connection prematurely without sending a status.
			if err == io.EOF {
				http.Error(w, "upload failed: server closed connection unexpectedly", http.StatusServiceUnavailable)
			} else {
				// Handle other gRPC errors
				st, ok := status.FromError(err)
				if ok {
					http.Error(w, fmt.Sprintf("upload processing failed: %s", st.Message()), grpcStatusToHTTP(st.Code()))
				} else {
					// Handle non-gRPC errors that might occur
					http.Error(w, fmt.Sprintf("upload failed with unexpected error: %v", err), http.StatusInternalServerError)
				}
			}
			return
		}

		// --- Send Final HTTP Response ---
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // 200 OK
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			// Log this error, as we can't send another HTTP error response
			fmt.Printf("ERROR: Failed to encode successful response to client: %v\n", err)
		}
	}
}
