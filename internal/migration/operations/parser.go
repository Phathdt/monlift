package operations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// OperationParser parses MongoDB shell commands into operations
type OperationParser struct{}

// NewOperationParser creates a new OperationParser
func NewOperationParser() *OperationParser {
	return &OperationParser{}
}

// convertMongoShellToJSON converts MongoDB shell format to valid JSON
func (p *OperationParser) convertMongoShellToJSON(input string) string {
	// Replace single quotes with double quotes
	input = strings.ReplaceAll(input, "'", "\"")

	// Regex to match unquoted property names
	re := regexp.MustCompile(`([{,]\s*)([a-zA-Z_$][a-zA-Z0-9_$]*)(\s*:)`)

	// Replace unquoted property names with quoted ones
	return re.ReplaceAllString(input, `$1"$2"$3`)
}

// Parse parses a MongoDB shell command into an Operation
func (p *OperationParser) Parse(cmd string) (Operation, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, nil
	}

	// Handle collection operations
	if strings.HasPrefix(cmd, "db.") {
		parts := strings.SplitN(cmd, ".", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid command format: %s", cmd)
		}

		// Extract operation and arguments
		operationWithArgs := parts[1]
		operation := strings.Split(operationWithArgs, "(")[0]

		// Extract arguments from command
		argsStart := strings.Index(cmd, "(")
		argsEnd := strings.LastIndex(cmd, ")")
		if argsStart == -1 || argsEnd == -1 {
			return nil, fmt.Errorf("invalid command format: %s", cmd)
		}
		argsStr := strings.TrimSpace(cmd[argsStart+1 : argsEnd])

		switch operation {
		case "createCollection":
			return p.parseCreateCollection(argsStr)
		default:
			// For other operations, we need the collection name
			if len(parts) < 3 {
				return nil, fmt.Errorf("invalid command format: %s", cmd)
			}
			collectionName := parts[1]
			operationWithArgs = parts[2]
			operation = strings.Split(operationWithArgs, "(")[0]

			switch operation {
			case "insertOne":
				return p.parseInsertOne(collectionName, argsStr)
			case "insertMany":
				return p.parseInsertMany(collectionName, argsStr)
			case "updateOne":
				return p.parseUpdateOne(collectionName, argsStr)
			case "updateMany":
				return p.parseUpdateMany(collectionName, argsStr)
			case "deleteOne":
				return p.parseDeleteOne(collectionName, argsStr)
			case "deleteMany":
				return p.parseDeleteMany(collectionName, argsStr)
			case "createIndex":
				return p.parseCreateIndex(collectionName, argsStr)
			case "dropIndex":
				return p.parseDropIndex(collectionName, argsStr)
			case "drop":
				return p.parseDropCollection(collectionName)
			case "aggregate":
				return p.parseAggregate(collectionName, argsStr)
			default:
				return nil, fmt.Errorf("unsupported operation: %s", operation)
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported command format: %s", cmd)
	}
}

func (p *OperationParser) parseCreateCollection(argsStr string) (Operation, error) {
	args := strings.SplitN(argsStr, ",", 2)
	collectionName := strings.Trim(args[0], `'"`)
	if collectionName == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	op := &CreateCollectionOperation{
		CollectionName: collectionName,
	}

	// Parse options if provided
	if len(args) > 1 {
		// Clean up the JSON string
		jsonStr := strings.TrimSpace(args[1])
		jsonStr = p.convertMongoShellToJSON(jsonStr)

		// Parse and re-encode to standardize format
		var jsonData interface{}
		if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}

		standardizedJSON, err := json.Marshal(jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to standardize JSON: %w", err)
		}

		// Get the raw map for direct access
		optsMap := make(map[string]interface{})
		if err := json.Unmarshal(standardizedJSON, &optsMap); err != nil {
			return nil, fmt.Errorf("failed to parse collection options: %w", err)
		}

		// Handle timeseries options
		if timeseries, ok := optsMap["timeseries"].(map[string]interface{}); ok {
			timeField, timeFieldOk := timeseries["timeField"].(string)
			if !timeFieldOk {
				return nil, fmt.Errorf("timeseries timeField must be a string")
			}

			// Create options
			tsOpts := &TimeSeriesOptions{
				TimeField: timeField,
			}

			// Handle optional fields
			if metaField, ok := timeseries["metaField"].(string); ok {
				tsOpts.MetaField = metaField
			}

			if granularity, ok := timeseries["granularity"].(string); ok {
				tsOpts.Granularity = granularity
			}

			op.TimeSeriesOpts = tsOpts
		}

		// For BSON parsing
		bsonOptsMap := bson.M{}
		if err := bson.UnmarshalExtJSON(standardizedJSON, true, &bsonOptsMap); err != nil {
			return nil, fmt.Errorf("failed to parse collection options as BSON: %w", err)
		}

		// Handle validator (handle any valid MongoDB validator expression)
		if validator, ok := bsonOptsMap["validator"]; ok {
			op.Validator = validator
		}

		// Handle validation options
		valOpts := &ValidatorOptions{}

		if validationLevel, ok := bsonOptsMap["validationLevel"].(string); ok {
			valOpts.ValidationLevel = validationLevel
		}

		if validationAction, ok := bsonOptsMap["validationAction"].(string); ok {
			valOpts.ValidationAction = validationAction
		}

		if valOpts.ValidationLevel != "" || valOpts.ValidationAction != "" {
			op.ValidatorOpts = valOpts
		}
	}

	return op, nil
}

func (p *OperationParser) parseInsertOne(collectionName, argsStr string) (Operation, error) {
	// Parse the document from the command
	doc := bson.M{}
	if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return &InsertOneOperation{
		CollectionName: collectionName,
		Document:       doc,
	}, nil
}

func (p *OperationParser) parseInsertMany(collectionName, argsStr string) (Operation, error) {
	// Parse the documents from the command
	docs := []interface{}{}
	if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &docs); err != nil {
		return nil, fmt.Errorf("failed to parse documents: %w", err)
	}

	return &InsertManyOperation{
		CollectionName: collectionName,
		Documents:      docs,
	}, nil
}

func (p *OperationParser) parseUpdateOne(collectionName, argsStr string) (Operation, error) {
	args := strings.Split(argsStr, ",")
	if len(args) < 2 {
		return nil, fmt.Errorf("invalid updateOne command format: %s", argsStr)
	}

	filter := bson.M{}
	update := bson.M{}

	if err := bson.UnmarshalExtJSON([]byte(args[0]), true, &filter); err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	if err := bson.UnmarshalExtJSON([]byte(args[1]), true, &update); err != nil {
		return nil, fmt.Errorf("failed to parse update: %w", err)
	}

	return &UpdateOneOperation{
		CollectionName: collectionName,
		Filter:         filter,
		Update:         update,
	}, nil
}

func (p *OperationParser) parseUpdateMany(collectionName, argsStr string) (Operation, error) {
	args := strings.Split(argsStr, ",")
	if len(args) < 2 {
		return nil, fmt.Errorf("invalid updateMany command format: %s", argsStr)
	}

	filter := bson.M{}
	update := bson.M{}

	if err := bson.UnmarshalExtJSON([]byte(args[0]), true, &filter); err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	if err := bson.UnmarshalExtJSON([]byte(args[1]), true, &update); err != nil {
		return nil, fmt.Errorf("failed to parse update: %w", err)
	}

	return &UpdateManyOperation{
		CollectionName: collectionName,
		Filter:         filter,
		Update:         update,
	}, nil
}

func (p *OperationParser) parseDeleteOne(collectionName, argsStr string) (Operation, error) {
	filter := bson.M{}
	if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &filter); err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	return &DeleteOneOperation{
		CollectionName: collectionName,
		Filter:         filter,
	}, nil
}

func (p *OperationParser) parseDeleteMany(collectionName, argsStr string) (Operation, error) {
	filter := bson.M{}
	if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &filter); err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	return &DeleteManyOperation{
		CollectionName: collectionName,
		Filter:         filter,
	}, nil
}

func (p *OperationParser) parseCreateIndex(collectionName, argsStr string) (Operation, error) {
	argsStr = p.convertMongoShellToJSON(argsStr)

	// Find the first closing brace that's not part of a nested object
	braceCount := 0
	splitPos := -1
	for i, char := range argsStr {
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				splitPos = i + 1
				break
			}
		}
	}

	if splitPos == -1 {
		return nil, fmt.Errorf("invalid JSON format in createIndex command: %s", argsStr)
	}

	// Split the string at the first closing brace
	keysStr := strings.TrimSpace(argsStr[:splitPos])
	optsStr := strings.TrimSpace(argsStr[splitPos+1:])
	if len(optsStr) > 0 && optsStr[0] == ',' {
		optsStr = strings.TrimSpace(optsStr[1:])
	}

	// Parse keys
	var keysData interface{}
	if err := json.Unmarshal([]byte(keysStr), &keysData); err != nil {
		return nil, fmt.Errorf("failed to parse index keys JSON: %w", err)
	}

	standardizedKeys, err := json.Marshal(keysData)
	if err != nil {
		return nil, fmt.Errorf("failed to standardize index keys JSON: %w", err)
	}

	keys := bson.M{}
	if err := bson.UnmarshalExtJSON(standardizedKeys, true, &keys); err != nil {
		return nil, fmt.Errorf("failed to parse index keys: %w", err)
	}

	// Check if this is a text index
	isTextIndex := false
	for _, v := range keys {
		if strVal, ok := v.(string); ok && strVal == "text" {
			isTextIndex = true
			break
		}
	}

	// Convert bson.M to bson.D for index keys
	indexKeys := bson.D{}
	for k, v := range keys {
		if num, ok := v.(float64); ok {
			indexKeys = append(indexKeys, bson.E{Key: k, Value: int32(num)})
		} else if str, ok := v.(string); ok && str == "text" {
			indexKeys = append(indexKeys, bson.E{Key: k, Value: "text"})
		} else {
			indexKeys = append(indexKeys, bson.E{Key: k, Value: v})
		}
	}

	// Create operation with keys
	op := &CreateIndexOperation{
		CollectionName: collectionName,
		Keys:           indexKeys,
	}

	// Parse options if any
	if optsStr != "" {
		var optsData interface{}
		if err := json.Unmarshal([]byte(optsStr), &optsData); err != nil {
			return nil, fmt.Errorf("failed to parse index options JSON: %w", err)
		}

		// Get the raw map for direct access to numeric values
		optsMap, ok := optsData.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to convert index options to map")
		}

		// Handle weights
		if weights, ok := optsMap["weights"].(map[string]interface{}); ok && isTextIndex {
			weightsD := bson.D{}
			for k, v := range weights {
				// Handle numeric weights directly from JSON to avoid BSON conversion issues
				if numVal, ok := v.(float64); ok {
					weightsD = append(weightsD, bson.E{Key: k, Value: int32(numVal)})
				} else if intVal, ok := v.(int); ok {
					weightsD = append(weightsD, bson.E{Key: k, Value: int32(intVal)})
				} else {
					// Last resort, try as string
					if strVal, ok := v.(string); ok {
						if intVal, err := strconv.ParseInt(strVal, 10, 32); err == nil {
							weightsD = append(weightsD, bson.E{Key: k, Value: int32(intVal)})
						}
					}
				}
			}

			if len(weightsD) > 0 {
				op.Weights = weightsD
			}
		}

		// Now convert to BSON for the other options
		standardizedOpts, err := json.Marshal(optsData)
		if err != nil {
			return nil, fmt.Errorf("failed to standardize index options JSON: %w", err)
		}

		// Parse the options
		indexOpts := bson.M{}
		if err := bson.UnmarshalExtJSON(standardizedOpts, true, &indexOpts); err != nil {
			return nil, fmt.Errorf("failed to parse index options: %w", err)
		}

		// Set partial filter expression if present
		if filter, ok := indexOpts["partialFilterExpression"]; ok {
			op.PartialFilter = filter
		}

		// Set other options
		if unique, ok := indexOpts["unique"].(bool); ok {
			op.Unique = unique
		}

		if sparse, ok := indexOpts["sparse"].(bool); ok {
			op.Sparse = sparse
		}

		if hidden, ok := indexOpts["hidden"].(bool); ok {
			op.Hidden = hidden
		}

		if name, ok := indexOpts["name"].(string); ok {
			op.Name = name
		}

		if defaultLanguage, ok := indexOpts["default_language"].(string); ok {
			op.DefaultLanguage = defaultLanguage
		}

		if expireAfterSeconds, ok := indexOpts["expireAfterSeconds"].(float64); ok {
			op.ExpireAfterSeconds = int32(expireAfterSeconds)
		} else if m, ok := indexOpts["expireAfterSeconds"].(bson.M); ok {
			if numIntStr, ok := m["$numberInt"].(string); ok {
				if intVal, err := strconv.ParseInt(numIntStr, 10, 32); err == nil {
					op.ExpireAfterSeconds = int32(intVal)
				}
			}
		} else if val, ok := indexOpts["expireAfterSeconds"].(int32); ok {
			op.ExpireAfterSeconds = val
		}

		if wildcardProjection, ok := indexOpts["wildcardProjection"].(bson.M); ok {
			projectionD := bson.D{}
			for k, v := range wildcardProjection {
				projectionD = append(projectionD, bson.E{Key: k, Value: v})
			}
			op.WildcardProjection = projectionD
		}

		if bucketSize, ok := indexOpts["bucketSize"].(int32); ok {
			op.BucketSize = bucketSize
		}
	}

	return op, nil
}

func (p *OperationParser) parseDropIndex(collectionName, argsStr string) (Operation, error) {
	indexName := strings.Trim(argsStr, `'"`)
	return &DropIndexOperation{
		CollectionName: collectionName,
		IndexName:      indexName,
	}, nil
}

func (p *OperationParser) parseDropCollection(collectionName string) (Operation, error) {
	return &DropCollectionOperation{
		CollectionName: collectionName,
	}, nil
}

func (p *OperationParser) parseAggregate(collectionName, argsStr string) (Operation, error) {
	pipeline := []bson.M{}
	if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline: %w", err)
	}

	return &AggregateOperation{
		CollectionName: collectionName,
		Pipeline:       pipeline,
	}, nil
}

// ParseScript parses a MongoDB script into a list of operations
func (p *OperationParser) ParseScript(script string) ([]Operation, error) {
	// Split the script into individual commands
	commands := strings.Split(script, ";")
	operations := make([]Operation, 0, len(commands))

	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}

		op, err := p.Parse(cmd)
		if err != nil {
			return nil, err
		}

		if op != nil {
			operations = append(operations, op)
		}
	}

	return operations, nil
}
