package main

import (
	"container/list"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

//go:embed resourceSchema/schema_gcp.json
var pulumiSchema []byte

const CodegenDir = "/CodegenDir/"
const TargetLanguage = "TS"

type UIType struct {
	typeName string
	desc     string
}

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

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line().Id(fieldName)

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

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line().Id(fieldName)

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
		resourceTypeEnum := Line()
		resourceTypeMap := map[string]*list.List{}
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

			if val, ok := resourceTypeMap[resourceFamily]; ok {
				val.PushBack(resourceName)
			} else {
				resourceTypeMap[resourceFamily] = list.New()
				resourceTypeMap[resourceFamily].PushBack(resourceName)
			}

			typeMap := map[string]UIType{}
			generator := Line().Line().Id("export").Id("interface").Id(resourceName + "Args").BlockFunc(func(g *Group) {

				for resourceField, fieldProperty := range resourceData.InputProperties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line()
					fieldStatement.Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					fType := GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()
					typeMap[fieldName] = UIType{
						typeName: fType,
						desc:     strings.Trim(fieldProperty.Description, "\n")}
				}

			})

			generator.Id("export").Id("class").Id(resourceName).Id("extends").Id("Resource").BlockFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.Properties {
					charArray := []rune(resourceField)
					fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line()
					fieldStatement.Id("public").Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()
				}
				g.Id("public").Id("static").Id("GetTypes()").Op(":").Id("DynamicUIProps[]").Id("{").Line().Id("return [")

				for k, v := range typeMap {
					g.Id("new DynamicUIProps(" + v.typeName + ",'" + k + "'," + strconv.Quote(v.desc) + "),")
				}
				g.Line().Id("];}")
			})

			importData := Line()

			importData.Id("import { InputType } from 'src/app/enum/InputType';").Line()
			importData.Id("import { Resource } from 'src/app/Models/CloudResource';").Line()
			importData.Id("import { DynamicUIProps } from 'src/app/components/resource-config/resource-config.component';").Line()
			for importName, _ := range importSet {
				importData.Id("import {").Id(importName).Op("} from").Id("'../types/" + importName + "';").Line()
			}

			fileContent := CleanTSCode(fmt.Sprintf("%#v", importData.Line().Add(generator)))

			fmt.Println("Writting to ", providerDir+resourceFamily+"/"+resourceName+".ts")
			if err = os.WriteFile(providerDir+resourceFamily+"/"+resourceName+".ts", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		importData := Line()
		importData.Id("import { ResourceType } from './ResourceType.ts';").Line()
		importData.Id("import { Resource } from 'src/app/Models/CloudResource';").Line()
		importData.Id("import { DynamicUIProps } from 'src/app/components/resource-config/resource-config.component';").Line()

		ResourceFactoryMap := Id("export class ResourceProperties {").Line().Id(" static readonly ResourceFactoryMap = new Map<ResourceType, () => Resource>([").Line()
		PropertiesMap := Line()
		for k, v := range resourceTypeMap {
			for i := v.Front(); i != nil; i = i.Next() {
				resourceEnum := strings.ToUpper(k) + "_" + strings.ToUpper(i.Value.(string))
				resourceTypeEnum.Id(resourceEnum).Op(",").Line()
				ResourceFactoryMap.Id("[ResourceType." + resourceEnum + ", () => new " + strings.ToUpper(k) + "_" + i.Value.(string) + "()],").Line()
				PropertiesMap.Id("[ResourceType." + resourceEnum + "," + strings.ToUpper(k) + "_" + i.Value.(string) + ".GetTypes()],").Line()
				//fmt.Println(k, i.Value.(string))
				importData.Id("import { " + i.Value.(string) + " as " + strings.ToUpper(k) + "_" + i.Value.(string) + " } from './" + k + "/" + i.Value.(string) + "';").Line()
			}

		}

		ResourceFactoryMap.Id("]);").Line()

		ResourceFactoryMap.Id("public static GetResourceObject(resType: ResourceType): Resource {	return (this.ResourceFactoryMap.get(resType) as () => Resource)();  }").Line()

		ResourceFactoryMap.Id(" public static propertiesMap: Map<ResourceType, DynamicUIProps[]> = new Map([").Line()
		ResourceFactoryMap.Add(PropertiesMap).Line().Id("]);}")

		resourceFactoryMapContent := CleanTSCode(fmt.Sprintf("%#v", importData.Line().Add(ResourceFactoryMap)))
		resourceTypeEnumContent := CleanTSCode(fmt.Sprintf("%#v", Id("export").Id("enum").Id("ResourceType").Block(resourceTypeEnum)))

		if err = os.WriteFile(providerDir+"/ResourceProperties.ts", []byte(resourceFactoryMapContent), 0644); err != nil {
			panic(err)
		}

		if err = os.WriteFile(providerDir+"/ResourceType.ts", []byte(resourceTypeEnumContent), 0644); err != nil {
			panic(err)
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

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line()
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
				importData.Id("import {").Id(importName).Op("} from").Id("'./" + importName + "';").Line()
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

func CleanTSCode(codeString string) string {
	_, fileContent, _ := strings.Cut(codeString, "\n")
	fileContent = strings.Trim(fileContent, ")")
	return fileContent
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

func GetResourceType(typeSpec *schema.TypeSpec, statement *Statement, importSet map[string]struct{}) string {
	fType := "InputType.String"
	switch typeSpec.Type {
	case "boolean":
		if TargetLanguage == "Go" {
			statement.Bool()
		} else if TargetLanguage == "TS" {
			statement.Id("boolean")
		}
		fType = "InputType.CheckBox"
		break

	case "integer":
		if TargetLanguage == "Go" {
			statement.Int()
		} else if TargetLanguage == "TS" {
			statement.Id("number")
		}

		fType = "InputType.Number"
		break

	case "number":
		if TargetLanguage == "Go" {
			statement.Float64()
		} else if TargetLanguage == "TS" {
			statement.Id("number")
		}

		fType = "InputType.Number"
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

		fType = "InputType.DropDown"
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

	return fType
}
