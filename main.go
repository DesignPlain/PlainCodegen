package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	. "github.com/dave/jennifer/jen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

//go:embed resourceSchema/schema_gcp.json
var pulumiSchema []byte

const CodegenDir = "/CodegenDir/"
const TargetLanguage = "Go"

func main() {
	codegenDir := CodegenDir + "go/"
	if TargetLanguage == "TS" {
		codegenDir = CodegenDir + "ts/"
	}

	var packageSpec schema.PackageSpec
	err := json.Unmarshal(pulumiSchema, &packageSpec)
	if err != nil {
		log.Fatalf("cannot deserialize schema: %v", err)
	}

	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	providerName := packageSpec.Name
	providerDir := rootDir + codegenDir + providerName + "/"
	providerResourceSubTypeDir := providerDir + "types/"

	fmt.Println("Writting to ", providerDir)
	if err := os.MkdirAll(providerResourceSubTypeDir, os.ModePerm); err != nil {
		panic(err)
	}

	if TargetLanguage == "Go" {
		for resourceURI, resourceData := range packageSpec.Resources {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			resourceFamily := strings.Split(resourceParts[1], "/")[0]

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			generator := NewFile(resourceFamily)
			generator.Type().Id(resourceName).StructFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.InputProperties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Trim(fieldProperty.Description, "\n")).Line().Id(fieldName)

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Tag((map[string]string{"json": resourceField + ",omitempty", "yaml": resourceField + ",omitempty"}))
					fieldStatement.Line()

				}
			})

			imp := Line()
			for importName, _ := range importSet {
				imp.Add(Id(importName).Id("\"../types/" + importName + "\"").Line())
			}

			importData := Line()
			if len(importSet) > 0 {
				importData = Id("import").Params(imp).Line()
			}

			fileContent := fmt.Sprintf("%#v", Id("package").Id(resourceFamily).Line().Add(importData).Add(generator))

			fmt.Println("Writting to ", providerDir+resourceFamily+"/"+resourceName+".go")
			if err = os.WriteFile(providerDir+resourceFamily+"/"+resourceName+".go", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		for resourceURI, resourceData := range packageSpec.Types {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			resourceFamily := strings.Split(resourceParts[1], "/")[0]

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			generator := NewFile(resourceFamily)
			generator.Type().Id(resourceName).StructFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.Properties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Trim(fieldProperty.Description, "\n")).Line().Id(fieldName)

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Tag((map[string]string{"json": resourceField + ",omitempty", "yaml": resourceField + ",omitempty"}))
					fieldStatement.Line()

				}
			})

			imp := Line()
			for importName, _ := range importSet {
				imp.Add(Id(importName).Id("\"./" + importName + "\"").Line())
			}

			importData := Line()
			if len(importSet) > 0 {
				importData = Id("import").Params(imp).Line()
			}

			fileContent := fmt.Sprintf("%#v", Id("package").Id("types").Line().Add(importData).Add(generator))
			fmt.Println("Writting to ", providerResourceSubTypeDir+resourceName+".go")
			if err = os.WriteFile(providerResourceSubTypeDir+resourceName+".go", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

	} else if TargetLanguage == "TS" {
		for resourceURI, resourceData := range packageSpec.Resources {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			resourceFamily := strings.Split(resourceParts[1], "/")[0]

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			generator := Id("export").Id("class").Id(resourceName).Id("extends").Id("Resource").BlockFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.Properties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Trim(fieldProperty.Description, "\n")).Line()
					fieldStatement.Id("public").Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()

				}
			})

			generator.Line().Line().Id("export").Id("interface").Id(resourceName + "Args").BlockFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.InputProperties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Trim(fieldProperty.Description, "\n")).Line()
					fieldStatement.Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()

				}
			})

			importData := Line()
			for importName, _ := range importSet {
				importData.Id("import").Op("{").Id(importName).Op("}").Id("from").Id("'../types/" + importName + "'").Op(";").Line()
			}

			_, fileContent, _ := strings.Cut(fmt.Sprintf("%#v", importData.Line().Add(generator)), "\n")
			fileContent = strings.Trim(fileContent, ")")

			fmt.Println("Writting to ", providerDir+resourceFamily+"/"+resourceName+".ts")
			if err = os.WriteFile(providerDir+resourceFamily+"/"+resourceName+".ts", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		for resourceURI, resourceData := range packageSpec.Types {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			resourceFamily := strings.Split(resourceParts[1], "/")[0]

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			generator := Line().Id("export").Id("interface").Id(resourceName).BlockFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.Properties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Trim(fieldProperty.Description, "\n")).Line()
					fieldStatement.Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()

				}
			})

			importData := Line()
			for importName, _ := range importSet {
				importData.Id("import").Op("{").Id(importName).Op("}").Id("from").Id("'./" + importName + "'").Op(";").Line()
			}

			_, fileContent, _ := strings.Cut(fmt.Sprintf("%#v", importData.Line().Add(generator)), "\n")
			fileContent = strings.Trim(fileContent, ")")

			fmt.Println("Writting to ", providerResourceSubTypeDir+resourceName+".ts")
			if err = os.WriteFile(providerResourceSubTypeDir+resourceName+".ts", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		FormatCode()
	}
}

func FormatCode() {
	var rootDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	err = os.Chdir(rootDir + "/utilities/typescript")
	if err != nil {
		panic(err)
	}

	if err != nil {
		panic(err)
	}

	codePath := rootDir + "/CodegenDir/"
	fmt.Println("Formatting dir: ", codePath)

	prettierCmd := exec.Command("npx", "prettier", codePath, "--write")
	output, err := prettierCmd.Output()
	if err != nil {
		fmt.Println("Prettier Error: ", err)
	}

	fmt.Println(string(output))
	err = os.Chdir(rootDir)
	if err != nil {
		panic(err)
	}
}

func GetResourceType(typeSpec *schema.TypeSpec, statement *Statement, importSet map[string]struct{}) {
	switch typeSpec.Type {
	case "boolean":
		if TargetLanguage == "Go" {
			statement.Bool()
		} else if TargetLanguage == "TS" {
			statement.Id("boolean")
		}
		break

	case "integer":
		if TargetLanguage == "Go" {
			statement.Int()
		} else if TargetLanguage == "TS" {
			statement.Id("number")
		}
		break

	case "number":
		if TargetLanguage == "Go" {
			statement.Float64()
		} else if TargetLanguage == "TS" {
			statement.Id("number")
		}
		break

	case "string":
		statement.String()
		break

	case "array":
		if TargetLanguage == "Go" {
			arrayStatement := statement.Index()
			GetResourceType(typeSpec.Items, arrayStatement, importSet)
		} else if TargetLanguage == "TS" {
			arrayStatement := statement.Id("Array").Op("<")
			GetResourceType(typeSpec.Items, arrayStatement, importSet)
			arrayStatement.Op(">")
		}
		break

	case "object":
		if TargetLanguage == "Go" {
			objectStatement := statement.Map(String())
			GetResourceType(typeSpec.AdditionalProperties, objectStatement, importSet)
		} else if TargetLanguage == "TS" {
			objectStatement := statement.Id("Map").Op("<").String().Op(",")
			GetResourceType(typeSpec.AdditionalProperties, objectStatement, importSet)
			objectStatement.Op(">")
		}
		break

	default:
		var typeName string
		// TODO analyse and add the custom pulumi types like Archive, Asset and json and any
		if !strings.Contains(typeSpec.Ref, "pulumi.json") {
			typeName = fmt.Sprintf("%s", strings.Split(typeSpec.Ref, ":")[2])
			importSet[typeName] = struct{}{}
		} else {
			typeName = "string"
		}

		statement.Id(typeName)
	}
}
