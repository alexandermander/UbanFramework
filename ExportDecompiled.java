import java.io.File;
import java.io.FileWriter;
import java.io.PrintWriter;
import java.util.ArrayList;
import java.util.List;
import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import ghidra.app.script.GhidraScript;
import ghidra.app.decompiler.*;
import ghidra.program.model.listing.CodeUnit;
import ghidra.program.model.listing.Data;
import ghidra.program.model.listing.DataIterator;
import ghidra.program.model.listing.*;
import ghidra.program.model.symbol.*;

public class ExportDecompiled extends GhidraScript {

    @Override
    protected void run() throws Exception {
        // Get the output path from the command line argument
        String[] args = getScriptArgs();
        if (args.length == 0) {
            printerr("Please provide an output path as a script argument.");
            return;
        }
        
        String outputPath = args[0];
        File outFile = new File(outputPath);
        File jsonFile = deriveJsonFile(outFile);

        // Initialize the Decompiler
        DecompInterface decomp = new DecompInterface();
        decomp.openProgram(currentProgram);

        Listing listing = currentProgram.getListing();
        List<GuidInfo> guidInfos = collectGuidInfos(listing);

        try (PrintWriter writer = new PrintWriter(outFile)) {
            writer.println("// Decompiled Code for: " + currentProgram.getName());
            writer.println("// Generated via Headless Ghidra\n");

            FunctionIterator iter = listing.getFunctions(true);

            while (iter.hasNext() && !monitor.isCancelled()) {
                Function f = iter.next();
                
                // Decompile each function (30-second timeout per function)
                DecompileResults results = decomp.decompileFunction(f, 300, monitor);
                
                if (results != null && results.decompileCompleted()) {
                    writer.println("/* Function: " + f.getName() + " at " + f.getEntryPoint() + " */");
                    writer.println(results.getDecompiledFunction().getC());
                    writer.println("\n// ------------------------------------------ \n");
                } else {
                    writer.println("// Failed to decompile function: " + f.getName());
                }
            }
        } finally {
            decomp.dispose();
        }

        writeGuidJson(jsonFile, guidInfos);
    }

    private static final class GuidInfo {
        private final String name;
        private final String address;
        private final String value;

        private GuidInfo(String name, String address, String value) {
            this.name = name;
            this.address = address;
            this.value = value;
        }
    }

    private static final class GuidPayload {
        private final String program;
        private final int guid_count;
        private final List<GuidInfo> guids;

        private GuidPayload(String program, List<GuidInfo> guids) {
            this.program = program;
            this.guid_count = guids.size();
            this.guids = guids;
        }
    }

    private List<GuidInfo> collectGuidInfos(Listing listing) {
        List<GuidInfo> infos = new ArrayList<>();
        DataIterator dataIter = listing.getDefinedData(true);

        while (dataIter.hasNext()) {
            Data data = dataIter.next();
            if (!"EFI_GUID".equals(data.getDataType().getName())) {
                continue;
            }

            Symbol primarySymbol = data.getPrimarySymbol();
            String guidName = primarySymbol != null ? primarySymbol.getName() : "<unnamed_guid>";
            String guidValue = data.getComment(CodeUnit.PLATE_COMMENT);
            if (guidValue != null && guidValue.isBlank()) {
                guidValue = null;
            }

            String address = "0x" + data.getAddress().toString().toUpperCase();
            infos.add(new GuidInfo(guidName, address, guidValue));
        }

        return infos;
    }

    private void writeGuidJson(File jsonFile, List<GuidInfo> infos) throws Exception {
        GuidPayload payload = new GuidPayload(currentProgram.getName(), infos);
        Gson gson = new GsonBuilder().setPrettyPrinting().create();
        try (FileWriter writer = new FileWriter(jsonFile)) {
            gson.toJson(payload, writer);
        }
    }

    private File deriveJsonFile(File outFile) {
        String name = outFile.getName();
        int dot = name.lastIndexOf('.');
        String base = (dot > 0) ? name.substring(0, dot) : name;
        File parent = outFile.getParentFile() != null ? outFile.getParentFile() : new File(".");
        return new File(parent, base + ".json");
    }

}
