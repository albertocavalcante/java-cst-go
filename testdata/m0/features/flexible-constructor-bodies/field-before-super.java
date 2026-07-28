class FlexibleFieldBoundary extends RuntimeException {
    int value;

    FlexibleFieldBoundary(int value) {
        this.value = value;
        super(Integer.toString(value));
    }
}
