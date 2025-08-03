package com.mtlog.analyzer

import com.intellij.DynamicBundle
import org.jetbrains.annotations.PropertyKey

private const val BUNDLE = "messages.MtlogBundle"

/**
 * Message bundle for plugin internationalization.
 */
object MtlogBundle : DynamicBundle(BUNDLE) {
    /**
     * Retrieves localized message.
     */
    @JvmStatic
    fun message(@PropertyKey(resourceBundle = BUNDLE) key: String, vararg params: Any): String =
        getMessage(key, *params)
}