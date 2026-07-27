final class SwitchPatternBoundary {
    static String describe(Object value) {
        return switch (value) {
            case String text -> text;
            default -> "other";
        };
    }
}
