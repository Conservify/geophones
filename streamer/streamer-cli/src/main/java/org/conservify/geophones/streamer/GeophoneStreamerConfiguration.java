package org.conservify.geophones.streamer;

import java.io.File;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.function.Predicate;

public class GeophoneStreamerConfiguration {
    private final SimpleDateFormat formatter = new SimpleDateFormat("yyyyMMdd_HHmmss");

    public int getSamplesPerFile() {
        return 512 * 60;
    }

    public int getBaudRate() {
        return 115200;
    }

    public long getActivityTimeout() {
        return 5 * 1000;
    }

    public String generateFileName() {
        return "geophone_" + formatter.format(new Date()) + ".bin";
    }

    public Predicate<File> getUploadPredicate() {
        return new Predicate<File>() {
            @Override
            public boolean test(File file) {
                return file.getName().contains("geophone_");
            }
        };
    }
}
