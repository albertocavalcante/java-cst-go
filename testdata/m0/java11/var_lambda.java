import java.util.function.Function;

final class VarLambda {
    Function<String, String> trim = (var value) -> value.trim();
}
