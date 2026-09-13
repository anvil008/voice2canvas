def _native_pipeline_impl(ctx):
    args = [f.path for f in ctx.outputs.outs] + [ctx.info_file.path]
    ctx.actions.run(
        executable = ctx.executable.script,
        arguments = args,
        inputs = depset(ctx.files.srcs + [ctx.info_file]),
        outputs = ctx.outputs.outs,
        mnemonic = "NativePipeline",
        progress_message = "Running CT160 pipeline %{label}",
        execution_requirements = {"no-remote-exec": "1", "no-sandbox": "1", "requires-network": "1"},
    )

native_pipeline = rule(
    implementation = _native_pipeline_impl,
    attrs = {
        "srcs": attr.label_list(allow_files = True),
        "script": attr.label(executable = True, cfg = "exec", allow_single_file = True),
        "outs": attr.output_list(mandatory = True),
    },
)
