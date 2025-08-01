package com.mtlog.goland

import com.intellij.DynamicBundle
import org.jetbrains.annotations.PropertyKey

object MtlogBundle : DynamicBundle("messages.MtlogBundle") {
    @JvmStatic
    fun message(@PropertyKey(resourceBundle = "messages.MtlogBundle") key: String, vararg params: Any): String {
        return getMessage(key, *params)
    }
}