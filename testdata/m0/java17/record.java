sealed interface Shape permits Circle {}

record Circle(double radius) implements Shape {}
