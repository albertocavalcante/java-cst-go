package example;

import java.util.List;
import static java.util.Collections.emptyList;

public class Sample {
    private int first = 1, second;

    static {
    }

    public Sample(String name, int... values) {
    }

    public List<String> values(int limit) {
        return emptyList();
    }

    interface Nested {
    }

    enum State {
        READY;

        int code;
    }
}

interface Service {
    int VALUE = 1;

    void run(String input);
}

enum Color {
    RED,
    GREEN;
}

@interface Marker {
    String value();
}
