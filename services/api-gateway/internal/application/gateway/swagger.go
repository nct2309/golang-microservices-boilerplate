package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

// RegisterSwaggerUI registers handlers for Swagger UI with the Fiber app
func (g *Gateway) RegisterSwaggerUI(swaggerDir string) {
	// Ensure swaggerDir exists
	if _, err := os.Stat(swaggerDir); os.IsNotExist(err) {
		g.logger.Warn("Swagger directory not found", "path", swaggerDir)
		return
	}

	// Create a merged swagger definition from the proto directory
	protoDir := path.Join(swaggerDir, "proto")
	if _, err := os.Stat(protoDir); !os.IsNotExist(err) {
		// Create a merged swagger definition
		mergedSwagger, err := mergeSwaggerFiles(g, protoDir)
		if err != nil {
			g.logger.Error("Failed to merge swagger files", "error", err)
		} else {
			// Parse descriptions from summaries if needed
			processDescriptionsAndDefaults(mergedSwagger)

			// Serve the merged swagger file
			g.app.Get("/swagger/openapi.json", func(c *fiber.Ctx) error {
				c.Set("Content-Type", "application/json")
				return c.JSON(mergedSwagger)
			})
			g.logger.Info("Registered merged swagger definition", "endpoint", "/swagger/openapi.json")
		}
	} else {
		g.logger.Info("Proto directory not found", "path", protoDir)

		// Fall back to serving individual swagger files from services directory
		servicesDir := path.Join(swaggerDir, "services")
		if _, err := os.Stat(servicesDir); !os.IsNotExist(err) {
			// Read all service directories
			serviceDirs, err := os.ReadDir(servicesDir)
			if err == nil {
				for _, serviceDir := range serviceDirs {
					if serviceDir.IsDir() {
						serviceName := serviceDir.Name()
						// Look for the specific pattern {serviceName}.swagger.json
						swaggerPath := path.Join(servicesDir, serviceName, serviceName+".swagger.json")

						// Only register if the swagger file exists
						if _, err := os.Stat(swaggerPath); !os.IsNotExist(err) {
							route := "/swagger/" + serviceName + "/openapi.json"
							g.app.Get(route, func(c *fiber.Ctx) error {
								return c.SendFile(swaggerPath)
							})
							g.logger.Info("Registered Swagger UI", "service", serviceName, "endpoint", route)
						} else {
							// Log if the expected file wasn't found, but don't stop registration
							g.logger.Warn("Expected swagger file not found", "path", swaggerPath)
						}
					}
				}
			} else {
				g.logger.Error("Failed to read services directory", "path", servicesDir, "error", err)
			}
		} else {
			g.logger.Warn("Services directory not found", "path", servicesDir)
		}
	}

	// Serve the Swagger UI static files
	swaggerUIRoot := http.Dir(path.Join(swaggerDir, "swagger-ui"))
	g.app.Use("/swagger/", filesystem.New(filesystem.Config{
		Root:   swaggerUIRoot,
		Browse: false, // Disable directory browsing
	}))

	// Redirect /swagger and /swagger/ to /swagger/index.html
	g.app.Get("/swagger", func(c *fiber.Ctx) error {
		return c.Redirect("/swagger/index.html", http.StatusMovedPermanently)
	})
	g.app.Get("/swagger/", func(c *fiber.Ctx) error {
		return c.Redirect("/swagger/index.html", http.StatusMovedPermanently)
	})

	g.logger.Info("Registered Swagger UI static files", "endpoint", "/swagger/")
}

// Extract description from summary fields and set it to the description field
func processDescriptionsAndDefaults(swagger map[string]interface{}) {
	// Process paths
	if paths, ok := swagger["paths"].(map[string]interface{}); ok {
		for _, pathItemRaw := range paths {
			if pathItem, ok := pathItemRaw.(map[string]interface{}); ok {
				// For each HTTP method in the path
				for _, methodRaw := range pathItem {
					if method, ok := methodRaw.(map[string]interface{}); ok {
						// Extract notes from summary
						if summary, ok := method["summary"].(string); ok {
							lines := strings.Split(summary, "\n")
							if len(lines) > 1 {
								// First line is the summary, rest is description
								method["summary"] = lines[0]

								// Join the rest as description
								description := strings.Join(lines[1:], "\n")
								if existingDesc, ok := method["description"].(string); ok {
									method["description"] = existingDesc + "\n" + description
								} else {
									method["description"] = description
								}
							}
						}

						// Process parameters
						if params, ok := method["parameters"].([]interface{}); ok {
							for _, paramRaw := range params {
								if param, ok := paramRaw.(map[string]interface{}); ok {
									// Extract default value and description from name
									if name, ok := param["name"].(string); ok && strings.Contains(name, " - ") {
										parts := strings.SplitN(name, " - ", 2)
										param["name"] = parts[0]

										if _, hasDesc := param["description"]; !hasDesc {
											param["description"] = parts[1]
										}
									}

									// Make default values and examples visible
									if schema, ok := param["schema"].(map[string]interface{}); ok {
										// Handle default values
										if defaultVal, hasDefault := schema["default"]; hasDefault {
											param["default"] = defaultVal
											// Also add as x-default for better visibility
											param["x-default"] = defaultVal
										}

										// Handle examples
										if example, hasExample := schema["example"]; hasExample {
											param["example"] = example
											// Also add as x-example for better visibility
											param["x-example"] = example
										}
									}
								}
							}
						}

						// Process request body
						if requestBody, ok := method["requestBody"].(map[string]interface{}); ok {
							if content, ok := requestBody["content"].(map[string]interface{}); ok {
								for _, mediaType := range content {
									if schema, ok := mediaType.(map[string]interface{})["schema"].(map[string]interface{}); ok {
										if props, ok := schema["properties"].(map[string]interface{}); ok {
											for _, propRaw := range props {
												if prop, ok := propRaw.(map[string]interface{}); ok {
													// Handle default values
													if defaultVal, hasDefault := prop["default"]; hasDefault {
														prop["x-default"] = defaultVal
													}

													// Handle examples
													if example, hasExample := prop["example"]; hasExample {
														prop["x-example"] = example
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Process definitions/schemas
	if definitions, ok := swagger["definitions"].(map[string]interface{}); ok {
		for _, defRaw := range definitions {
			if def, ok := defRaw.(map[string]interface{}); ok {
				if props, ok := def["properties"].(map[string]interface{}); ok {
					for _, propRaw := range props {
						if prop, ok := propRaw.(map[string]interface{}); ok {
							// Make default values and examples visible
							if defaultVal, hasDefault := prop["default"]; hasDefault {
								prop["x-default"] = defaultVal
							}

							if example, hasExample := prop["example"]; hasExample {
								prop["x-example"] = example
							}

							// Handle enum descriptions
							if enum, ok := prop["enum"].([]interface{}); ok {
								if _, hasDesc := prop["description"]; !hasDesc {
									prop["description"] = "Allowed values: " + fmt.Sprint(enum)
								}
							}
						}
					}
				}
			}
		}
	}
}

// mergeSwaggerFiles finds and merges all swagger.json files in the proto directory
func mergeSwaggerFiles(g *Gateway, protoDir string) (map[string]interface{}, error) {
	// Initialize the merged swagger definition
	mergedSwagger := map[string]interface{}{
		"swagger": "2.0",
		"info": map[string]interface{}{
			"title":       "Microservices API",
			"version":     "1.0",
			"description": "Merged API for all microservices",
		},
		"schemes":     []string{"http", "https"}, // Specify HTTP scheme
		"paths":       map[string]interface{}{},
		"definitions": map[string]interface{}{},
		"tags":        []interface{}{},
		"consumes":    []interface{}{},
		"produces":    []interface{}{},
	}

	// Find all swagger.json files in the proto directory and its subdirectories
	var swaggerFiles []string
	err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".swagger.json") {
			swaggerFiles = append(swaggerFiles, path)
			g.logger.Info("Found swagger file", "path", path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Maps to track existing tags for deduplication
	existingTags := make(map[string]bool)
	existingSecurityDefs := make(map[string]bool)

	// Merge each swagger file into the merged definition
	for _, file := range swaggerFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			g.logger.Error("Failed to read swagger file", "path", file, "error", err)
			continue
		}

		var swagger map[string]interface{}
		if err := json.Unmarshal(data, &swagger); err != nil {
			g.logger.Error("Failed to parse swagger file", "path", file, "error", err)
			continue
		}

		// Merge paths
		if paths, ok := swagger["paths"].(map[string]interface{}); ok {
			mergedPaths := mergedSwagger["paths"].(map[string]interface{})
			for path, pathDef := range paths {
				// Check if this path already exists in the merged swagger
				if _, exists := mergedPaths[path]; exists {
					// Path already exists, log a warning and continue
					g.logger.Warn("Path already exists in merged swagger, skipping", "path", path)
					continue
				}
				mergedPaths[path] = pathDef
			}
		}

		// Merge definitions
		if definitions, ok := swagger["definitions"].(map[string]interface{}); ok {
			mergedDefs := mergedSwagger["definitions"].(map[string]interface{})
			for def, defObj := range definitions {
				// Check if this definition already exists in the merged swagger
				if _, exists := mergedDefs[def]; exists {
					// Definition already exists, log a warning but don't overwrite
					g.logger.Warn("Definition already exists in merged swagger, skipping", "definition", def)
					continue
				}
				mergedDefs[def] = defObj
			}
		}

		// Merge tags (making sure to avoid duplicates)
		if tags, ok := swagger["tags"].([]interface{}); ok {
			mergedTags := mergedSwagger["tags"].([]interface{})
			for _, tag := range tags {
				if tagMap, ok := tag.(map[string]interface{}); ok {
					if name, ok := tagMap["name"].(string); ok {
						if !existingTags[name] {
							mergedTags = append(mergedTags, tag)
							existingTags[name] = true
						}
					}
				}
			}
			mergedSwagger["tags"] = mergedTags
		}

		// Merge consumes
		if consumes, ok := swagger["consumes"].([]interface{}); ok {
			mergedConsumes := mergedSwagger["consumes"].([]interface{})
			mergedConsumesMap := make(map[string]bool)

			// Add existing consumes to map for deduplication
			for _, item := range mergedConsumes {
				if strItem, ok := item.(string); ok {
					mergedConsumesMap[strItem] = true
				}
			}

			// Add new consumes
			for _, item := range consumes {
				if strItem, ok := item.(string); ok {
					if !mergedConsumesMap[strItem] {
						mergedConsumes = append(mergedConsumes, strItem)
						mergedConsumesMap[strItem] = true
					}
				}
			}
			mergedSwagger["consumes"] = mergedConsumes
		}

		// Merge produces
		if produces, ok := swagger["produces"].([]interface{}); ok {
			mergedProduces := mergedSwagger["produces"].([]interface{})
			mergedProducesMap := make(map[string]bool)

			// Add existing produces to map for deduplication
			for _, item := range mergedProduces {
				if strItem, ok := item.(string); ok {
					mergedProducesMap[strItem] = true
				}
			}

			// Add new produces
			for _, item := range produces {
				if strItem, ok := item.(string); ok {
					if !mergedProducesMap[strItem] {
						mergedProduces = append(mergedProduces, strItem)
						mergedProducesMap[strItem] = true
					}
				}
			}
			mergedSwagger["produces"] = mergedProduces
		}

		// Merge securityDefinitions
		if securityDefs, ok := swagger["securityDefinitions"].(map[string]interface{}); ok {
			var mergedSecDefs map[string]interface{}

			// Initialize if not already present
			if existing, ok := mergedSwagger["securityDefinitions"].(map[string]interface{}); ok {
				mergedSecDefs = existing
			} else {
				mergedSecDefs = make(map[string]interface{})
				mergedSwagger["securityDefinitions"] = mergedSecDefs
			}

			// Add new security definitions
			for name, def := range securityDefs {
				if !existingSecurityDefs[name] {
					mergedSecDefs[name] = def
					existingSecurityDefs[name] = true
				}
			}
		}

		// Copy any additional fields from the first swagger file as a fallback
		for key, value := range swagger {
			if _, exists := mergedSwagger[key]; !exists {
				mergedSwagger[key] = value
			}
		}
	}

	return mergedSwagger, nil
}
