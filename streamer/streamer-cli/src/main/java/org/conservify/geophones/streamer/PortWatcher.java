package org.conservify.geophones.streamer;

import com.google.common.collect.Lists;
import jssc.SerialPort;
import jssc.SerialPortList;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Collections;
import java.util.Dictionary;
import java.util.Hashtable;
import java.util.List;

public class PortWatcher {
    private static final Logger logger = LoggerFactory.getLogger(PortWatcher.class);
    private final Dictionary<String, Streamer> streamers = new Hashtable<>();
    private final GeophoneStreamerConfiguration configuration;

    public PortWatcher(GeophoneStreamerConfiguration configuration) {
        this.configuration = configuration;
    }

    public void find() {
        String[] portNames = SerialPortList.getPortNames();

        List<String> inactive = Lists.newArrayList();
        inactive.addAll(Collections.list(streamers.keys()));

        for (String portName : portNames) {
            if (streamers.get(portName) == null) {
                logger.info("Creating streamer for {}", portName);
                createStreamer(portName);
            }
            else {
                if (!streamers.get(portName).check()) {
                    createStreamer(portName);
                }
                inactive.remove(portName);
            }
        }

        for (String portName : inactive) {
            logger.info("Removing streamer for {}", portName);
            streamers.get(portName).stop();
            streamers.remove(portName);
        }
    }

    private void createStreamer(String portName) {
        Streamer streamer = new Streamer(new SerialPort(portName), new GeophoneListener(new GeophoneWriter(configuration)));
        if (streamer.start()) {
            streamers.put(portName, streamer);
        }
    }
}
