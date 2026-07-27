final class SwitchExpressionBoundary {
    static String describe(int value) {
        return switch (value) {
            case 0 -> "zero";
            default -> "other";
        };
    }
}
