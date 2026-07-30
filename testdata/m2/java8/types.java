@Deprecated
package example.types;

import java.util.List;
import static java.util.Collections.*;

@Deprecated
public final class Types<T extends Number & Comparable<T>>
        extends Base
        implements java.io.Serializable, Cloneable {
    @Deprecated
    private java.util.Map<String, ? extends Number>[] values;
    private int count;
    private boolean enabled;
    private double ratio;

    public <U extends T> List<? super U> convert(
            final List<? extends T> input,
            U... rest
    ) throws java.io.IOException, IllegalArgumentException {
        return null;
    }

    public void reset() {
    }
}
