package main

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

const CodegenDir = "/CodegenDir/"

var ProviderFamily string = ""
var TargetLanguage string = ""
var SchemaFile string = ""
var SchemaType = ""

type UIType struct {
	typeName             string
	desc                 string
	subType              string
	isRequired           bool
	willReplaceOnChanges bool
}

func main() {

	if len(os.Args) != 3 {
		fmt.Println("Provide valid args - Codegen [TS|Go] schema_file_name")
		return
	}

	TargetLanguage = os.Args[1]
	SchemaFile := os.Args[2]

	if TargetLanguage != "TS" && TargetLanguage != "Go" {
		fmt.Println("Provide valid language [TS|Go]")
		return
	}

	if strings.Contains(SchemaFile, "aws") {
		ProviderFamily = "AWS"
	}

	codegenDir := CodegenDir + "go/"
	if TargetLanguage == "TS" {
		codegenDir = CodegenDir + "ts/"
	}

	pulumiSchema, err := os.ReadFile(SchemaFile)
	if err != nil {
		panic(err)
	}

	var packageSpec schema.PackageSpec
	err = json.Unmarshal(pulumiSchema, &packageSpec)
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
		resourceTypeEnum := Line()
		resourceTypeMap := map[string][]string{}

		stringFunc := Line()
		stringFunc.Id("func Get_" + strings.ToUpper(providerName) + "_String(rType ResourceType) string { \nswitch rType {\n")

		typeMap := Line()
		typeMap.Id("var ResourceTypeMap = map[ResourceType]func() Any{\n")

		for resourceURI, resourceData := range packageSpec.Resources {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			resourceFamily := strings.Split(resourceParts[1], "/")[0]
			resourceFamily = strings.Replace(resourceFamily, ".", "__", -1)

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			if val, ok := resourceTypeMap[resourceFamily]; ok {
				val = append(val, resourceName)
				slices.SortFunc(val, func(a, b string) int {
					return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
				})

				resourceTypeMap[resourceFamily] = val
			} else {
				resourceTypeMap[resourceFamily] = []string{resourceName}
			}

			charArray := []rune(resourceFamily)
			fieldName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])
			resourceTypeName := fieldName

			charArray = []rune(resourceName)
			fieldNameResName := strings.ToUpper(string(charArray[0])) + string(charArray[1:])
			resourceTypeName += "_" + fieldNameResName

			stringFunc.Id("case " + strings.ToUpper(resourceTypeName) + ":\n return " + "\"" + resourceTypeName + "\"\n")
			typeMap.Id(strings.ToUpper(resourceTypeName) + ": func() Any {\n return &" + resourceFamily + "." + fieldNameResName + "{}\n},\n")

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

			// imp := Line()
			// for importName := range importSet {
			// 	imp.Add(Id(importName).Id("\"../types/" + importName + "\"").Line())
			// }

			importData := Line()
			if len(importSet) > 0 {
				// importData = Id("import").Params(imp).Line()
				importData.Id("import types \"libds/" + providerName + "/types\"")
			}

			fileContent := fmt.Sprintf("%#v", Id("package").Id(resourceFamily).Line().Add(importData).Add(generator))

			fmt.Println("Writting to ", providerDir+resourceFamily+"/"+resourceName+".go")
			if err = os.WriteFile(providerDir+resourceFamily+"/"+resourceName+".go", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		keys := make([]string, 0, len(resourceTypeMap))
		for k := range resourceTypeMap {
			keys = append(keys, k)
		}

		// Sort ResourceName
		slices.SortFunc(keys, func(a, b string) int {
			return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
		})

		res_import := Line()

		count := 0
		for _, res_family := range keys {
			res_import.Id("\"libds/" + providerName + "/" + res_family + "\"").Line()
			for _, resourceName := range resourceTypeMap[res_family] {
				count += 1
				res_family_upper := strings.ToUpper(res_family)
				resourceEnum := res_family_upper + "_" + strings.ToUpper(resourceName)
				resourceTypeEnum.Id(resourceEnum)
				if count == 1 && strings.Contains(providerName, "aws") {
					resourceTypeEnum.Id(" ResourceType = RESOURCE_TYPE_LIMIT*1 + iota")
				} else if count == 1 && strings.Contains(providerName, "gcp") {
					resourceTypeEnum.Id(" ResourceType = RESOURCE_TYPE_LIMIT*0 + iota")
				}
				resourceTypeEnum.Line()
			}
		}

		SchemaType = "Types"
		for resourceURI, resourceData := range packageSpec.Types {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceFamily := strings.Split(resourceParts[1], "/")[0]
			resourceFamily = strings.Replace(resourceFamily, ".", "__", -1)
			resourceName := resourceFamily + "_" + resourceParts[2]

			charArray := []rune(resourceName)
			resourceName = strings.ToUpper(string(charArray[0])) + string(charArray[1:])

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
			for importName := range importSet {
				imp.Add(Id(importName).Id("\"./" + importName + "\"").Line())
			}

			fileContent := fmt.Sprintf("%#v", Id("package").Id("types").Line().Add(generator))
			//fmt.Println("Writting to ", providerResourceSubTypeDir+resourceName+".go")
			if err = os.WriteFile(providerResourceSubTypeDir+resourceName+".go", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		stringFunc.Id("\n }\n return \"Unknown ResourceType\"\n}\n")

		typeMap.Id("}")

		res_import.Id(`. "libds/ds_base"`)

		fileContent := fmt.Sprintf("%#v", Id("package").Id(providerName).Line().Id("import (").Add(res_import).Id(")").Line().Id("const (").Add(resourceTypeEnum).Id(")").Line().Add(stringFunc).Line().Add(typeMap))

		fmt.Println("Writting to ", providerDir+"resource_type.go")
		if err = os.WriteFile(providerDir+"resource_type.go", []byte(fileContent), 0644); err != nil {
			panic(err)
		}

	} else if TargetLanguage == "TS" {
		type ResDetails struct {
			res_name string
			desc     string
		}

		resourceTypeEnum := Line()
		resourceTypeMap := map[string][]ResDetails{}
		for resourceURI, resourceData := range packageSpec.Resources {
			importSet := map[string]struct{}{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			if resourceName == "Map" {
				resourceName = "MapResource"
			}

			resourceSubParts := strings.Split(resourceParts[1], "/")
			resourceFamily := resourceSubParts[0]

			if providerName == "kubernetes" && len(resourceSubParts) >= 2 {
				api_version := resourceSubParts[1]
				resourceName += "__" + api_version
			}

			resourceFamily = strings.Replace(resourceFamily, ".", "__", -1)

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			before, _, present := strings.Cut(resourceData.Description, "## Example Usage")
			if !present {
				before, _, _ = strings.Cut(resourceData.Description, "\n")
			}

			before = strings.Replace(before, "\n", " ", -1)
			before = strings.Replace(before, "\r", " ", -1)
			before = strings.Replace(before, "\"", "\\\"", -1)

			res_desc := strings.Replace(before, "*", "-", -1)
			res_desc = strings.TrimRight(res_desc, "\n ")

			res_detail := ResDetails{
				res_name: resourceName,
				desc:     res_desc,
			}

			if val, ok := resourceTypeMap[resourceFamily]; ok {
				val = append(val, res_detail)
				slices.SortFunc(val, func(a, b ResDetails) int {
					return cmp.Compare(strings.ToLower(a.res_name), strings.ToLower(b.res_name))
				})

				resourceTypeMap[resourceFamily] = val
			} else {
				resourceTypeMap[resourceFamily] = []ResDetails{res_detail}
			}

			typeMap := map[string]UIType{}
			generator := Line().Line().Id("export").Id("interface").Id(resourceName + "Args").BlockFunc(func(g *Group) {

				for resourceField, fieldProperty := range resourceData.InputProperties {
					fieldName := resourceField

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line()
					fieldStatement.Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					fType := GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()

					//fmt.Println("object" + fType)
					if strings.Contains(fType, "ARRX") {

						fType = strings.Replace(fType, "InputType.", "InputType_", 1)
						typeMap[fieldName] = UIType{
							typeName: "InputType.Array",
							subType:  strings.TrimLeft(strings.TrimLeft(fType, "ARRX"), "OBX") + "_GetTypes()",
						}
					} else if strings.Contains(fType, "OBX") {

						fType = strings.Replace(fType, "InputType.", "InputType_", 1)

						typeMap[fieldName] = UIType{
							typeName: "InputType.Object",
							subType:  strings.TrimLeft(fType, "OBX") + "_GetTypes()",
						}
					} else if strings.Contains(fType, "InputType.Map") {
						typeMap[fieldName] = UIType{
							typeName: "InputType.Map",
							subType:  "InputType_Map_GetTypes()",
						}
					} else {
						typeMap[fieldName] = UIType{
							typeName: fType,
						}
					}

					res := typeMap[fieldName]
					res.willReplaceOnChanges = fieldProperty.WillReplaceOnChanges
					res.desc = strings.Trim(fieldProperty.Description, "\n")
					typeMap[fieldName] = res

				}
			})

			for _, fieldName := range resourceData.RequiredInputs {
				res := typeMap[fieldName]
				res.isRequired = true
				typeMap[fieldName] = res

			}

			//fmt.Printf("Required Inp: %#v\n", typeMap)

			importData := Line()
			generator.Id("export").Id("class").Id(resourceName).Id("extends").Id("DS_Resource").BlockFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.Properties {
					fieldName := resourceField

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
					if v.typeName == "InputType.Object" || v.typeName == "InputType.Array" || v.typeName == "InputType.Map" {
						g.Id("new DynamicUIProps(" + v.typeName + ",'" + k + "'," + strconv.Quote(v.desc) + "," + "() => " + v.subType + "," + strconv.FormatBool(v.isRequired) + "," + strconv.FormatBool(v.willReplaceOnChanges) + "),")
					} else {
						g.Id("new DynamicUIProps(" + v.typeName + ",'" + k + "'," + strconv.Quote(v.desc) + "," + "() => []" + "," + strconv.FormatBool(v.isRequired) + "," + strconv.FormatBool(v.willReplaceOnChanges) + "),")
					}
				}

				g.Line().Id("];}")
			})

			importData.Id("import { InputType, InputType_String_GetTypes, InputType_Number_GetTypes, InputType_Map_GetTypes } from '../../ds_base/InputType';").Line()
			importData.Id("import { DS_Resource } from '../../ds_base/Resource';").Line()
			importData.Id("import { DynamicUIProps } from '../../ds_base/DynamicUIProps';").Line()
			for importName := range importSet {
				importData.Id("import {").Id(importName).Id(",").Id(importName + "_GetTypes").Op("} from").Id("'../types/" + importName + "';").Line()
			}

			fileContent := CleanTSCode(fmt.Sprintf("%#v", importData.Line().Add(generator)))

			fmt.Println("Writting to ", providerDir+resourceFamily+"/"+resourceName+".ts")
			if err = os.WriteFile(providerDir+resourceFamily+"/"+resourceName+".ts", []byte(fileContent), 0644); err != nil {
				panic(err)
			}
		}

		// Setup ResourceProperties.ts file
		importData := Line()
		importData.Id("import { ResourceType } from './ResourceType';").Line()
		importData.Id("import { DS_Resource, ResourceProperty } from '../ds_base/Resource';").Line()
		importData.Id("import { DynamicUIProps } from '../ds_base/DynamicUIProps';").Line()

		ResourceFactoryMap := Id("export class ResourceProperties {").Line()

		PropertiesMap := Line()

		GetObj := Line()

		keys := make([]string, 0, len(resourceTypeMap))
		for k := range resourceTypeMap {
			keys = append(keys, k)
		}

		// Sort ResourceName
		slices.SortFunc(keys, func(a, b string) int {
			return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
		})

		count := 0
		for _, res_family := range keys {
			for _, resourceDetail := range resourceTypeMap[res_family] {
				// TS has some issue when we have more than 1001 elements in map when statically set
				if count%1001 == 0 {
					mapCount := strconv.Itoa(count/1001 + 1)

					if count/1001 != 0 {
						ResourceFactoryMap.Id("]);").Line()
						PropertiesMap.Id("]);").Line()
						GetObj.Id(`
						if (map == undefined) {
							map = this.ResourceFactoryMap2.get(resType)
						}`).Line()
					} else {
						GetObj.Id("let map = this.ResourceFactoryMap1.get(resType)")
					}

					ResourceFactoryMap.Id(" static readonly ResourceFactoryMap" + mapCount + " = new Map<ResourceType, () => DS_Resource>([").Line()
					PropertiesMap.Id("public static propertiesMap" + mapCount + ": Map<ResourceType, ResourceProperty> = new Map([").Line()
				}

				count += 1

				//fmt.Println(resourceName)
				res_family_upper := strings.ToUpper(res_family)
				resourceEnum := res_family_upper + "_" + strings.ToUpper(resourceDetail.res_name)
				resourceTypeEnum.Id(resourceEnum)
				if count == 1 && ProviderFamily == "AWS" {
					resourceTypeEnum.Id(" = 5000")
				}
				resourceTypeEnum.Op(",").Line()

				ResourceFactoryMap.Id("[ResourceType." + resourceEnum + ", () => new " + res_family_upper + "_" + resourceDetail.res_name + "()],").Line()
				PropertiesMap.Id("[ResourceType." + resourceEnum + ", new ResourceProperty(\"" + resourceDetail.desc + "\"," + res_family_upper + "_" + resourceDetail.res_name + ".GetTypes())],").Line()
				importData.Id("import { " + resourceDetail.res_name + " as " + res_family_upper + "_" + resourceDetail.res_name + " } from './" + res_family + "/" + resourceDetail.res_name + "';").Line()
			}
		}

		ResourceFactoryMap.Id("]);").Line()
		ResourceFactoryMap.Id(`
		  public static GetResourceObject(resType: ResourceType): DS_Resource {`)
		ResourceFactoryMap.Add(GetObj)
		ResourceFactoryMap.Id(`
    		return (map as () => DS_Resource)();
  		}`).Line()

		ResourceFactoryMap.Add(PropertiesMap).Line().Id("]);}")

		resourceFactoryMapContent := CleanTSCode(fmt.Sprintf("%#v", importData.Line().Add(ResourceFactoryMap)))
		if err = os.WriteFile(providerDir+"/ResourceProperties.ts", []byte(resourceFactoryMapContent), 0644); err != nil {
			panic(err)
		}

		resourceTypeEnumContent := CleanTSCode(fmt.Sprintf("%#v", Id("export").Id("enum").Id("ResourceType").Block(resourceTypeEnum)))
		if err = os.WriteFile(providerDir+"/ResourceType.ts", []byte(resourceTypeEnumContent), 0644); err != nil {
			panic(err)
		}

		// Setup types/*.ts files
		for resourceURI, resourceData := range packageSpec.Types {
			importSet := map[string]struct{}{}

			typeMap := map[string]UIType{}

			resourceParts := strings.Split(resourceURI, ":")
			resourceName := resourceParts[2]
			resourceSubParts := strings.Split(resourceParts[1], "/")
			resourceFamily := resourceSubParts[0]
			resourceFamily = strings.Replace(resourceFamily, ".", "__", -1)

			if providerName == "kubernetes" && len(resourceSubParts) > 2 {
				api_version := resourceSubParts[1]
				resourceName += "__" + api_version
			} else {
				resourceName = resourceFamily + "_" + resourceName
			}

			_, err = os.Stat(providerDir + resourceFamily)
			if err != nil {
				if err := os.Mkdir(providerDir+resourceFamily, os.ModePerm); err != nil {
					panic(err)
				}
			}

			generator := Line().Id("export").Id("interface").Id(resourceName).BlockFunc(func(g *Group) {
				for resourceField, fieldProperty := range resourceData.Properties {

					fieldName := resourceField

					fieldStatement := g.Commentf(strings.Replace(strings.Trim(fieldProperty.Description, "\n"), "*", "-", -1)).Line()
					fieldStatement.Id(fieldName).Op("?:")

					// TODO: temporary hack, will write a converted for this
					var targetStruct *schema.TypeSpec
					temporaryVariable, _ := json.Marshal(fieldProperty)
					err = json.Unmarshal(temporaryVariable, &targetStruct)

					fType := GetResourceType(targetStruct, fieldStatement, importSet)

					fieldStatement.Op(";").Line()

					// fmt.Println("object" + fType)
					if strings.Contains(fType, "ARRX") {
						fType = strings.Replace(fType, "InputType.", "InputType_", 1)

						typeMap[fieldName] = UIType{
							typeName: "InputType.Array",
							subType:  strings.TrimLeft(strings.TrimLeft(fType, "ARRX"), "OBX") + "_GetTypes()",
						}
					} else if strings.Contains(fType, "OBX") {
						fType = strings.Replace(fType, "InputType.", "InputType_", 1)
						typeMap[fieldName] = UIType{
							typeName: "InputType.Object",
							subType:  strings.TrimLeft(fType, "OBX") + "_GetTypes()",
						}
					} else if strings.Contains(fType, "InputType.Map") {
						typeMap[fieldName] = UIType{
							typeName: "InputType.Map",

							subType: "InputType_Map_GetTypes()",
						}
					} else {
						typeMap[fieldName] = UIType{
							typeName: fType,
						}
					}

					res := typeMap[fieldName]
					res.willReplaceOnChanges = fieldProperty.WillReplaceOnChanges
					res.desc = strings.Trim(fieldProperty.Description, "\n")
					typeMap[fieldName] = res

				}

				for _, fieldName := range resourceData.Required {
					res := typeMap[fieldName]
					res.isRequired = true
					typeMap[fieldName] = res
				}

				//fmt.Printf("Required Complex Types: %#v\n", typeMap)

				g.Op("}").Line()

				g.Id("export").Id("function").Id(resourceName + "_GetTypes()").Op(":").Id("DynamicUIProps[]").Id("{").Line().Id("return [")

				for k, v := range typeMap {
					if v.typeName == "InputType.Object" || v.typeName == "InputType.Array" || v.typeName == "InputType.Map" {
						g.Id("new DynamicUIProps(" + v.typeName + ",'" + k + "'," + strconv.Quote(v.desc) + "," + "() => " + v.subType + "," + strconv.FormatBool(v.isRequired) + "," + strconv.FormatBool(v.willReplaceOnChanges) + "),")
					} else {
						g.Id("new DynamicUIProps(" + v.typeName + ",'" + k + "'," + strconv.Quote(v.desc) + "," + "() => []" + "," + strconv.FormatBool(v.isRequired) + "," + strconv.FormatBool(v.willReplaceOnChanges) + "),")
					}
				}
				g.Line().Id("];")
			})

			importData := Line()
			importData.Id("import { InputType, InputType_String_GetTypes, InputType_Number_GetTypes, InputType_Map_GetTypes } from '../../ds_base/InputType';").Line()
			importData.Id("import { DynamicUIProps } from '../../ds_base/DynamicUIProps';").Line()
			for importName := range importSet {
				importData.Id("import {").Id(importName).Id(",").Id(importName + "_GetTypes").Op("} from").Id("'./" + importName + "';").Line()
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

	codePath := rootDir + "/CodegenDir/ts"
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
	//by, _ := json.Marshal(typeSpec)
	//fmt.Println("[Debug]: Resource Type -> %#v", string(by))

	if len(typeSpec.OneOf) > 0 {
		typeSpec_temp := &typeSpec.OneOf[0]
		if typeSpec_temp.Type != "string" && len(typeSpec.OneOf) > 1 {
			typeSpec_temp = &typeSpec.OneOf[1]
		}

		typeSpec = typeSpec_temp
	}

	if typeSpec.Type == "object" && typeSpec.Ref != "" {
		typeSpec.Type = "ref"
	}

	switch typeSpec.Type {
	case "boolean":
		if TargetLanguage == "Go" {
			statement.Bool()
		} else if TargetLanguage == "TS" {
			statement.Id("boolean")
		}
		fType = "InputType.Bool"

	case "integer":
		if TargetLanguage == "Go" {
			statement.Int()
		} else if TargetLanguage == "TS" {
			statement.Id("number")
		}

		fType = "InputType.Number"

	case "number":
		if TargetLanguage == "Go" {
			statement.Float64()
		} else if TargetLanguage == "TS" {
			statement.Id("number")
		}

		fType = "InputType.Number"

	case "string":
		statement.String()

	case "array":
		if TargetLanguage == "Go" {
			arrayStatement := statement.Index()
			GetResourceType(typeSpec.Items, arrayStatement, importSet)
		} else if TargetLanguage == "TS" {
			arrayStatement := statement.Id("Array").Op("<")
			fType := GetResourceType(typeSpec.Items, arrayStatement, importSet)
			arrayStatement.Op(">")

			return "ARRX" + fType

		}

	case "object":
		if TargetLanguage == "Go" {
			objectStatement := statement.Map(String())
			GetResourceType(typeSpec.AdditionalProperties, objectStatement, importSet)
		} else if TargetLanguage == "TS" {
			objectStatement := statement.Id("Map").Op("<").String().Op(",")
			// TODO: if value is not `string` type (like pulumi.json#/Any) modify the fType, will need to generate the Dynamic UI accordingly
			GetResourceType(typeSpec.AdditionalProperties, objectStatement, importSet)
			objectStatement.Op(">")
		}

		fType = "InputType.Map"

	default:
		//fmt.Println("[Debug]: Resource Type -> ", typeSpec.Ref)
		var typeName string
		// TODO analyse and add the custom pulumi types like Archive, Asset and json and any
		if !strings.Contains(typeSpec.Ref, "pulumi.json") {
			resourceParts := strings.Split(typeSpec.Ref, ":")
			resourceFamily := strings.Split(resourceParts[1], "/")[0]
			resourceFamily = strings.Replace(resourceFamily, ".", "__", -1)
			typeName = resourceFamily + "_" + resourceParts[2]

			if TargetLanguage == "Go" {
				charArray := []rune(typeName)
				typeName = strings.ToUpper(string(charArray[0])) + string(charArray[1:])
			}
			fType = typeName

			importSet[typeName] = struct{}{}

			if TargetLanguage == "Go" && SchemaType != "Types" {
				typeName = "types." + typeName
			}
		} else {
			fType = "InputType.String"
			typeName = "string"
		}

		statement.Id(typeName)

		fType = "OBX " + fType
	}

	return fType
}
