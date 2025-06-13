import io
from proto_schema_parser.parser import Parser
from proto_schema_parser.generator import Generator
# https://github.com/criccomini/proto-schema-parser


text = io.open("./proto-template/templates/device/v1/deviceapis_device_dtmf_v1.proto", mode="r", encoding="utf-8").read()

result = Parser().parse(text)

result.file_elements[11].text = "// dgdfddhsdj\n// asadaewewd"
proto = Generator().generate(result)
print(proto)
