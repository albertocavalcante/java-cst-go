class FlexibleConstructorBoundary extends RuntimeException {
    FlexibleConstructorBoundary(int value) {
        if (value < 0) {
            throw new IllegalArgumentException();
        }
        super(Integer.toString(value));
    }
}
