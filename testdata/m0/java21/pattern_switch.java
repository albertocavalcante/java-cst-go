final class PatternSwitch {
    static String describe(Object value) {
        return switch (value) {
            case Integer number -> "integer " + number;
            case String text when !text.isEmpty() -> text;
            default -> "other";
        };
    }
}
