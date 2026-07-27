final class PrimitivePatternBoundary {
    static String describe(Object value) {
        return switch (value) {
            case int number -> "int " + number;
            default -> "other";
        };
    }
}
