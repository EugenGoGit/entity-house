import com.google.inject.Guice;
import com.google.inject.Injector;
import io.protostuff.compiler.ParserModule;
import io.protostuff.compiler.model.Message;
import io.protostuff.compiler.model.Proto;
import io.protostuff.compiler.model.Service;
import io.protostuff.compiler.parser.FileReader;
import io.protostuff.compiler.parser.FileReaderFactory;
import io.protostuff.compiler.parser.Importer;
import io.protostuff.compiler.parser.ProtoContext;

import java.nio.file.Path;
import java.util.List;

public final class Program {
  public static void main(final String[] args) {
    final Injector injector = Guice.createInjector(new ParserModule());

    final FileReaderFactory fileReaderFactory = injector.getInstance(FileReaderFactory.class);
    final List<Path> includePaths = List.of(
      Path.of("proto-template"),
      Path.of("proto-template/proto_deps"),
      Path.of("proto-template/templates/device/v1")
    );
    final FileReader fileReader = fileReaderFactory.create(includePaths);

    final Importer importer = injector.getInstance(Importer.class);
    final ProtoContext protoContext = importer.importFile(
      fileReader,
      "deviceapis_device_dtmf_v1.proto"
    );

    final Proto proto = protoContext.getProto();
    Message m = new Message(proto);
    m.setName("ttttt");
    m.setComments(List.of("кекекпкпап"));
    proto.addMessage(m);
    proto.addService(new Service(proto));



    final List<Message> messages = proto.getMessages();
    System.out.println(String.format("Messages: %s", messages));
    System.out.println(String.format("Messages: %s", proto.getCommentLines()));
  }
}
