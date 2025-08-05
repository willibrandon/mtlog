package com.mtlog.analyzer.quickfix

import com.goide.psi.GoCallExpr
import com.goide.psi.GoStringLiteral
import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.util.PsiTreeUtil
import com.mtlog.analyzer.logging.MtlogLogger
import com.mtlog.analyzer.notification.MtlogNotificationService
import com.goide.psi.*

/**
 * Quick fix to add missing error parameter when template contains {Error} or {Err}.
 */
class MissingErrorQuickFix(
    element: PsiElement? = null
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {
    
    init {
        // Log when quick fix is created
        element?.project?.let { project ->
            MtlogLogger.info("MissingErrorQuickFix CREATED for element: ${element.javaClass.simpleName}, text: '${element.text.take(50)}'", project)
        }
    }
    
    override fun getText(): String {
        val text = "Add error parameter"
        MtlogLogger.info("MissingErrorQuickFix.getText() called, returning: $text", startElement?.project)
        return text
    }
    
    override fun getFamilyName(): String = "mtlog"
    
    override fun isAvailable(project: Project, file: PsiFile, startElement: PsiElement, endElement: PsiElement): Boolean {
        MtlogLogger.info("MissingErrorQuickFix.isAvailable called", project)
        MtlogLogger.info("  startElement: ${startElement.javaClass.simpleName}, valid=${startElement.isValid}, text='${startElement.text.take(30)}'", project)
        MtlogLogger.info("  endElement: ${endElement.javaClass.simpleName}, valid=${endElement.isValid}", project)
        MtlogLogger.info("  file: ${file.name}, valid=${file.isValid}", project)
        
        val available = super.isAvailable(project, file, startElement, endElement)
        MtlogLogger.info("  super.isAvailable returned: $available", project)
        return available
    }
    
    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        MtlogLogger.info("===== MissingErrorQuickFix.invoke CALLED =====", project)
        MtlogLogger.info("startElement: ${startElement.javaClass.simpleName}, text: '${startElement.text.take(50)}'...", project)
        MtlogLogger.info("file: ${file.name}", project)
        
        // Find the call expression first
        val callExpr = when {
            startElement is GoCallExpr -> startElement
            else -> PsiTreeUtil.getParentOfType(startElement, GoCallExpr::class.java)
        }
        
        if (callExpr == null) {
            MtlogLogger.warn("Could not find GoCallExpr from startElement: ${startElement.javaClass.simpleName}", project)
            return
        }
        
        MtlogLogger.debug("Found call expression: ${callExpr.text}", project)
        
        // Find the string literal within the call expression
        val goStringLiteral = PsiTreeUtil.findChildOfType(callExpr, GoStringLiteral::class.java)
        if (goStringLiteral == null) {
            MtlogLogger.warn("Could not find GoStringLiteral in call expression", project)
            return
        }
        
        val doc = editor?.document 
            ?: PsiDocumentManager.getInstance(project).getDocument(file) 
            ?: return
        
        // Get template text
        val templateText = goStringLiteral.text
        val templateContent = when {
            templateText.startsWith("\"") && templateText.endsWith("\"") -> 
                templateText.substring(1, templateText.length - 1)
            templateText.startsWith("`") && templateText.endsWith("`") -> 
                templateText.substring(1, templateText.length - 1)
            else -> return
        }
        
        MtlogLogger.debug("Template content: '$templateContent'", project)
        
        // The actual modification logic
        val runnable = Runnable {
            try {
                // For MTLOG006, we just need to add an error parameter
                // Don't modify the template - that's the user's choice
                
                // Find existing error variable in scope or add nil
                val errorVar = findErrorVariableInScope(callExpr)
                val errorParam = errorVar ?: "nil"
                
                // Add error parameter as a separate argument
                val argList = callExpr.argumentList
                val expressions = argList.expressionList
                
                // Find where to insert the error parameter
                // It should be after the last argument
                val insertPos = if (expressions.isNotEmpty()) {
                    expressions.last().textRange.endOffset
                } else {
                    // No arguments? This shouldn't happen for MTLOG006
                    MtlogLogger.warn("No arguments found in call expression", project)
                    return@Runnable
                }
                
                MtlogLogger.debug("Inserting ', $errorParam' at position $insertPos", project)
                
                // Calculate line number before inserting
                val lineNumber = doc.getLineNumber(insertPos)
                
                doc.insertString(insertPos, ", $errorParam")
                
                // If we used nil, add a TODO comment (but check for existing comments first)
                if (errorParam == "nil") {
                    val lineStartOffset = doc.getLineStartOffset(lineNumber)
                    val lineEndOffset = doc.getLineEndOffset(lineNumber)
                    val lineText = doc.text.substring(lineStartOffset, lineEndOffset)
                    
                    // Check if there's already a comment on this line
                    val hasComment = lineText.contains("//")
                    
                    if (hasComment) {
                        // Put the TODO on the next line with proper indentation
                        val indentMatch = Regex("^(\\s*)").find(lineText)
                        val indent = indentMatch?.value ?: ""
                        MtlogLogger.debug("Line already has comment, adding TODO on next line with indent: '${indent}'", project)
                        doc.insertString(lineEndOffset, "\n$indent// TODO: replace nil with actual error")
                    } else {
                        // No existing comment, add at end of line
                        MtlogLogger.debug("Adding TODO comment at line end offset $lineEndOffset", project)
                        doc.insertString(lineEndOffset, " // TODO: replace nil with actual error")
                    }
                }
                
                MtlogLogger.debug("Committing document changes", project)
                PsiDocumentManager.getInstance(project).commitDocument(doc)
                
                // Save file and force re-analysis
                ApplicationManager.getApplication().invokeLater {
                    MtlogLogger.debug("Saving file to disk", project)
                    doc.let {
                        val fileDocManager = com.intellij.openapi.fileEditor.FileDocumentManager.getInstance()
                        fileDocManager.saveDocument(it)
                    }
                    
                    MtlogLogger.debug("Forcing re-analysis of file", project)
                    com.intellij.codeInsight.daemon.DaemonCodeAnalyzer.getInstance(project).restart(file)
                }
            } catch (e: Exception) {
                MtlogLogger.error("Error applying quick fix", project, e)
            }
        }
        
        executeWithAppropriateWriteAction(project, file, runnable)
    }
    
    /**
     * Executes the given runnable with the appropriate write action context.
     * Always wraps in WriteCommandAction if not already in a write action to avoid threading issues.
     */
    private fun executeWithAppropriateWriteAction(project: Project, file: PsiFile, runnable: Runnable) {
        val app = ApplicationManager.getApplication()
        MtlogLogger.debug("executeWithAppropriateWriteAction: writeAccessAllowed=${app.isWriteAccessAllowed}, readAccessAllowed=${app.isReadAccessAllowed}", project)
        
        // Always wrap in WriteCommandAction if not already in write action
        if (!app.isWriteAccessAllowed) {
            WriteCommandAction.runWriteCommandAction(project, getText(), null, runnable, file)
        } else {
            runnable.run()
        }
    }
    
    /**
     * Finds an error variable in the current scope.
     * Prioritizes finding the closest error variable to the log statement.
     */
    private fun findErrorVariableInScope(element: PsiElement): String? {
        MtlogLogger.info("Looking for error variables, element: ${element.text.take(30)}", element.project)
        
        // First priority: Check if we're inside an if statement that checks an error
        val parentIf = PsiTreeUtil.getParentOfType(element, GoIfStatement::class.java)
        MtlogLogger.info("Parent if statement: ${parentIf?.text?.take(50) ?: "null"}", element.project)
        if (parentIf != null) {
            val errorVarFromIf = getErrorVariableFromIfCondition(parentIf)
            MtlogLogger.info("Error variable from if condition: $errorVarFromIf", element.project)
            if (errorVarFromIf != null && isVariableInScope(element, errorVarFromIf)) {
                MtlogLogger.info("Found error variable '$errorVarFromIf' from parent if condition and it's in scope", element.project)
                return errorVarFromIf
            } else if (errorVarFromIf != null) {
                MtlogLogger.info("Error variable '$errorVarFromIf' from parent if condition is not in scope", element.project)
            }
        }
        
        // Find the containing function or method
        val function = PsiTreeUtil.getParentOfType(element, GoFunctionOrMethodDeclaration::class.java)
            ?: return null
        
        MtlogLogger.info("Looking for error variables in function: ${function.name}", element.project)
        
        // Second priority: Look for any error-typed variables in the immediate surrounding block
        val closestErrorVar = findClosestErrorVariable(element)
        if (closestErrorVar != null) {
            MtlogLogger.info("Found closest error variable: $closestErrorVar", element.project)
            return closestErrorVar
        }
        
        // Third priority: Fallback to common error variable names
        // But first check if errors are being explicitly ignored in recent statements
        if (hasRecentErrorIgnorance(element)) {
            MtlogLogger.info("Recent error ignorance detected, not using any error variables", element.project)
            return null
        }
        
        // Also check if we're in a different logical context (different if branch checking different variable)
        if (isInDifferentLogicalContext(element)) {
            MtlogLogger.info("In different logical context, not using error variables from other contexts", element.project)
            return null
        }
        
        val errorVarNames = listOf("err", "e", "error")
        
        // Check if any of these variables are in scope at the call site
        for (varName in errorVarNames) {
            MtlogLogger.info("Checking if common error variable '$varName' is in scope", element.project)
            if (isVariableInScope(element, varName)) {
                MtlogLogger.info("Found common error variable in scope: $varName", element.project)
                return varName
            } else {
                MtlogLogger.info("Common error variable '$varName' is not in scope", element.project)
            }
        }
        
        MtlogLogger.info("No error variable found in scope", element.project)
        return null
    }
    
    /**
     * Finds the closest error variable to the given element.
     * Looks for variables that are likely to be errors based on their type or usage context.
     */
    private fun findClosestErrorVariable(element: PsiElement): String? {
        MtlogLogger.info("findClosestErrorVariable called for element: ${element.text.take(30)}", element.project)
        
        // First, check if we're inside an if statement that checks an error
        val parentIf = PsiTreeUtil.getParentOfType(element, GoIfStatement::class.java)
        if (parentIf != null) {
            val errorVarFromIf = getErrorVariableFromIfCondition(parentIf)
            if (errorVarFromIf != null) {
                MtlogLogger.debug("Found error variable '$errorVarFromIf' in immediate parent if statement", element.project)
                return errorVarFromIf
            }
        }
        
        // Find the containing block
        val block = PsiTreeUtil.getParentOfType(element, GoBlock::class.java) ?: return null
        MtlogLogger.info("Containing block found, checking for error variables", element.project)
        MtlogLogger.info("Block parent: ${block.parent?.javaClass?.simpleName}", element.project)
        
        // Find the statement containing our element
        val ourStatement = PsiTreeUtil.getParentOfType(element, GoStatement::class.java)
        val ourOffset = ourStatement?.textRange?.startOffset ?: element.textRange.startOffset
        MtlogLogger.info("Our statement: ${ourStatement?.text?.take(50)}", element.project)
        
        // Collect all error variables defined before our statement IN THE SAME BLOCK
        val errorVars = mutableListOf<Pair<String, Int>>() // (varName, distance from our statement)
        
        val statements = block.statementList
        for (statement in statements) {
            // Stop when we reach or pass our statement
            if (statement.textRange.startOffset >= ourOffset) break
            
            // Skip if statements that we're not inside
            if (statement is GoIfStatement && !PsiTreeUtil.isAncestor(statement, element, false)) {
                MtlogLogger.info("Skipping if statement we're not inside: ${statement.text.take(30)}", element.project)
                continue
            }
            
            // Check for error variables in this statement
            val varsInStatement = findErrorVariablesInStatement(statement)
            for (varName in varsInStatement) {
                // Before adding, check if this variable is actually in scope
                if (isVariableInScope(element, varName)) {
                    val distance = ourOffset - statement.textRange.endOffset
                    errorVars.add(varName to distance)
                    MtlogLogger.info("Found error variable '$varName' at distance $distance (in scope)", element.project)
                } else {
                    MtlogLogger.info("Found error variable '$varName' but it's not in scope, skipping", element.project)
                }
            }
        }
        
        // Return the closest error variable
        val closest = errorVars.minByOrNull { it.second }?.first
        MtlogLogger.info("Closest error variable: $closest", element.project)
        return closest
    }
    
    /**
     * Extracts error variable from an if statement condition.
     */
    private fun getErrorVariableFromIfCondition(ifStatement: GoIfStatement): String? {
        val condition = ifStatement.condition
        MtlogLogger.info("If condition: ${condition?.text}", ifStatement.project)
        if (condition == null) return null
        
        if (condition.text.contains("!= nil")) {
            MtlogLogger.info("Condition contains '!= nil'", ifStatement.project)
            // The condition itself might be the binary expression
            val binaryExpr = when (condition) {
                is GoBinaryExpr -> condition
                else -> PsiTreeUtil.findChildOfType(condition, GoBinaryExpr::class.java)
            }
            MtlogLogger.info("Binary expression: ${binaryExpr?.text}", ifStatement.project)
            if (binaryExpr != null) {
                val leftExpr = binaryExpr.left
                MtlogLogger.info("Left expression type: ${leftExpr?.javaClass?.simpleName}, text: ${leftExpr?.text}", ifStatement.project)
                if (leftExpr is GoReferenceExpression) {
                    val varName = leftExpr.identifier?.text
                    MtlogLogger.info("Found variable name: $varName", ifStatement.project)
                    // Only return if this looks like an error variable
                    if (varName != null && isLikelyErrorVariable(varName)) {
                        MtlogLogger.info("Variable '$varName' appears to be an error variable", ifStatement.project)
                        return varName
                    } else {
                        MtlogLogger.info("Variable '$varName' does not appear to be an error variable, skipping", ifStatement.project)
                    }
                }
            }
        }
        
        return null
    }
    
    /**
     * Checks if a variable name is likely to be an error variable.
     */
    private fun isLikelyErrorVariable(varName: String): Boolean {
        val errorNames = setOf("err", "error", "e", "errs", "errors")
        return errorNames.contains(varName.lowercase()) || 
               varName.endsWith("Err") || 
               varName.endsWith("Error") ||
               varName.startsWith("err") ||
               varName.startsWith("error")
    }
    
    /**
     * Checks if there's a recent statement that explicitly ignores errors (uses _).
     */
    private fun hasRecentErrorIgnorance(element: PsiElement): Boolean {
        val function = PsiTreeUtil.getParentOfType(element, GoFunctionOrMethodDeclaration::class.java)
            ?: return false
        
        val block = function.block ?: return false
        val statements = block.statementList
        
        val ourStatement = PsiTreeUtil.getParentOfType(element, GoStatement::class.java)
        val ourOffset = ourStatement?.textRange?.startOffset ?: element.textRange.startOffset
        
        // Look at the last few statements before our current position
        val recentStatements = statements.filter { 
            it.textRange.startOffset < ourOffset 
        }.takeLast(3) // Check last 3 statements
        
        for (statement in recentStatements) {
            MtlogLogger.info("Checking statement for error ignorance: ${statement.text.take(50)}", element.project)
            MtlogLogger.info("Statement type: ${statement.javaClass.simpleName}", element.project)
            
            // Log all children to understand structure
            statement.children.forEach { child ->
                MtlogLogger.info("  Child: ${child.javaClass.simpleName} - ${child.text.take(30)}", element.project)
            }
            
            // Look for patterns like "_, _ = someFunc()" 
            val shortVarDecls = PsiTreeUtil.findChildrenOfType(statement, GoShortVarDeclaration::class.java)
            for (shortVarDecl in shortVarDecls) {
                val varDefs = shortVarDecl.varDefinitionList
                MtlogLogger.info("Found ${varDefs.size} variables in declaration: ${varDefs.map { it.name }}", element.project)
                if (varDefs.size >= 2 && varDefs.last().name == "_") {
                    MtlogLogger.info("Found error ignorance in statement: ${statement.text.take(50)}", element.project)
                    return true
                }
            }
            
            // Check if the statement itself is an assignment with blank identifiers
            if (statement is GoAssignmentStatement) {
                val leftExprs = statement.leftHandExprList.expressionList
                MtlogLogger.info("Direct assignment statement found with ${leftExprs.size} left-hand expressions: ${leftExprs.map { it.text }}", element.project)
                if (leftExprs.size >= 2 && leftExprs.last().text == "_") {
                    MtlogLogger.info("Found error ignorance in assignment: ${statement.text.take(50)}", element.project)
                    return true
                }
            }
            
            // Also check for assignment statements nested inside
            val assignments = PsiTreeUtil.findChildrenOfType(statement, GoAssignmentStatement::class.java)
            MtlogLogger.info("Found ${assignments.size} nested assignments in statement", element.project)
            for (assignment in assignments) {
                val leftExprs = assignment.leftHandExprList.expressionList
                MtlogLogger.info("Found ${leftExprs.size} left-hand expressions in nested assignment: ${leftExprs.map { it.text }}", element.project)
                if (leftExprs.size >= 2 && leftExprs.last().text == "_") {
                    MtlogLogger.info("Found error ignorance in nested assignment: ${statement.text.take(50)}", element.project)
                    return true
                }
            }
            
            // Check if the statement is a simple statement containing an assignment
            if (statement is GoSimpleStatement) {
                // Check for assignment statements directly
                val assignment = PsiTreeUtil.findChildOfType(statement, GoAssignmentStatement::class.java)
                if (assignment != null) {
                    val leftExprs = assignment.leftHandExprList.expressionList
                    MtlogLogger.info("Found ${leftExprs.size} left-hand expressions in simple statement assignment: ${leftExprs.map { it.text }}", element.project)
                    if (leftExprs.size >= 2 && leftExprs.last().text == "_") {
                        MtlogLogger.info("Found error ignorance in simple statement assignment: ${statement.text.take(50)}", element.project)
                        return true
                    }
                }
            }
        }
        
        MtlogLogger.info("No recent error ignorance found", element.project)
        return false
    }
    
    /**
     * Finds all error variables defined in a statement.
     */
    private fun findErrorVariablesInStatement(statement: GoStatement): List<String> {
        val errorVars = mutableListOf<String>()
        
        // IMPORTANT: Only look for declarations at the top level of this statement
        // Don't look inside nested blocks (if statements, etc.)
        
        // For if statements with init expressions, check the init part
        if (statement is GoIfStatement) {
            val initStatement = statement.statement
            if (initStatement != null) {
                // Check short var declarations in the init statement
                if (initStatement is GoShortVarDeclaration) {
                    val rightExpr = initStatement.rightExpressionsList?.firstOrNull()
                    if (rightExpr != null) {
                        val callExpr = PsiTreeUtil.findChildOfType(rightExpr, GoCallExpr::class.java)
                        if (callExpr != null) {
                            val varDefs = initStatement.varDefinitionList
                            if (varDefs.size >= 2) {
                                varDefs.lastOrNull()?.name?.let { errorVars.add(it) }
                            }
                        }
                    }
                }
            }
            // Don't look inside the if block itself
            return errorVars
        }
        
        // For simple statements, check short variable declarations
        if (statement is GoSimpleStatement) {
            val shortVarDecl = statement.shortVarDeclaration
            if (shortVarDecl != null) {
                val rightExpr = shortVarDecl.rightExpressionsList?.firstOrNull()
                if (rightExpr != null) {
                    val callExpr = PsiTreeUtil.findChildOfType(rightExpr, GoCallExpr::class.java)
                    if (callExpr != null) {
                        val varDefs = shortVarDecl.varDefinitionList
                        if (varDefs.size >= 2) {
                            varDefs.lastOrNull()?.name?.let { errorVars.add(it) }
                        }
                    }
                    
                    if (rightExpr.text.contains("Errorf") || rightExpr.text.contains("errors.New")) {
                        shortVarDecl.varDefinitionList.firstOrNull()?.name?.let { errorVars.add(it) }
                    }
                }
            }
        }
        
        return errorVars
    }
    
    /**
     * Checks if a variable with the given name is in scope at the element's position.
     */
    private fun isVariableInScope(element: PsiElement, varName: String): Boolean {
        MtlogLogger.info("isVariableInScope: checking if '$varName' is in scope for element: ${element.text.take(30)}", element.project)
        
        val function = PsiTreeUtil.getParentOfType(element, GoFunctionOrMethodDeclaration::class.java)
            ?: return false
        
        // Check function parameters
        function.signature?.parameters?.parameterDeclarationList?.forEach { param ->
            param.paramDefinitionList.forEach { paramDef ->
                if (paramDef.name == varName) {
                    MtlogLogger.info("Found $varName as function parameter", element.project)
                    return true
                }
            }
        }
        
        // Check if the variable is accessible from the current position
        // We need to walk up the tree and check each block scope
        var current: PsiElement? = element
        while (current != null && current != function) {
            val block = when (current) {
                is GoBlock -> current
                else -> PsiTreeUtil.getParentOfType(current, GoBlock::class.java, false)
            }
            
            if (block != null) {
                MtlogLogger.info("Checking block for variable '$varName' (block parent: ${block.parent?.javaClass?.simpleName})", element.project)
                // Check variables declared in this block before our position
                if (isVariableInBlock(block, element, varName)) {
                    MtlogLogger.info("Variable '$varName' found in current block", element.project)
                    return true
                }
                
                // Move to the parent of the block's parent (skip the statement containing the block)
                // This prevents variables from sibling blocks from being considered in scope
                val blockParent = block.parent
                if (blockParent is GoIfStatement || blockParent is GoForStatement || blockParent is GoSwitchStatement) {
                    // Skip this statement and go to its parent
                    current = blockParent.parent
                    MtlogLogger.info("Skipping block parent: ${blockParent.javaClass.simpleName}", element.project)
                    continue
                }
            }
            
            current = current.parent
        }
        
        MtlogLogger.info("Variable $varName not found in scope", element.project)
        return false
    }
    
    /**
     * Checks if a variable is declared in a specific block before the given element.
     */
    private fun isVariableInBlock(block: GoBlock, element: PsiElement, varName: String): Boolean {
        val statements = block.statementList
        
        for (statement in statements) {
            // Stop when we reach or pass the current element
            if (statement.textRange.startOffset >= element.textRange.startOffset) {
                break
            }
            
            // Skip if the statement contains our element (avoid checking inside the same statement)
            if (statement.textRange.contains(element.textRange)) {
                continue
            }
            
            // Check for if statements with init expressions (like if err := foo(); err != nil)
            if (statement is GoIfStatement) {
                val initStatement = statement.statement
                if (initStatement is GoShortVarDeclaration) {
                    // Only consider this variable if we're inside this if statement
                    if (PsiTreeUtil.isAncestor(statement, element, false)) {
                        initStatement.varDefinitionList.forEach { varDef ->
                            if (varDef.name == varName) {
                                MtlogLogger.info("Found $varName in if init statement (we're inside this if)", element.project)
                                return true
                            }
                        }
                    } else {
                        MtlogLogger.info("Skipping $varName from if init - we're not inside this if statement", element.project)
                    }
                }
                // Don't look inside the if block - variables there are not in our scope
                continue
            }
            
            // Check for short variable declarations
            val shortVarDecls = PsiTreeUtil.findChildrenOfType(statement, GoShortVarDeclaration::class.java)
            for (shortVarDecl in shortVarDecls) {
                // Make sure this declaration is not inside a nested block
                val declBlock = PsiTreeUtil.getParentOfType(shortVarDecl, GoBlock::class.java, false)
                if (declBlock == block) {
                    shortVarDecl.varDefinitionList.forEach { varDef ->
                        if (varDef.name == varName) {
                            MtlogLogger.info("Found $varName in short var declaration: ${shortVarDecl.text.take(50)}", element.project)
                            return true
                        }
                    }
                }
            }
            
            // Check var declarations
            val varSpecs = PsiTreeUtil.findChildrenOfType(statement, GoVarSpec::class.java)
            for (varSpec in varSpecs) {
                // Make sure this declaration is not inside a nested block
                val declBlock = PsiTreeUtil.getParentOfType(varSpec, GoBlock::class.java, false)
                if (declBlock == block) {
                    varSpec.varDefinitionList.forEach { varDef ->
                        if (varDef.name == varName) {
                            MtlogLogger.info("Found $varName in var declaration", element.project)
                            return true
                        }
                    }
                }
            }
        }
        
        return false
    }
    
    /**
     * Checks if we're in a different logical context where error variables from other contexts shouldn't be used.
     * For example, if we're in an if statement checking conn != nil, we shouldn't use err from a previous context.
     */
    private fun isInDifferentLogicalContext(element: PsiElement): Boolean {
        val parentIf = PsiTreeUtil.getParentOfType(element, GoIfStatement::class.java)
        if (parentIf == null) {
            return false
        }
        
        val condition = parentIf.condition
        if (condition == null) {
            return false
        }
        
        // Check if the condition is checking a non-error variable
        val conditionText = condition.text
        MtlogLogger.info("Checking logical context, parent if condition: $conditionText", element.project)
        
        // If we're checking a non-error variable (like conn, resp, etc.) we're in a different context
        if (conditionText.contains("!= nil") || conditionText.contains("== nil")) {
            val binaryExpr = when (condition) {
                is GoBinaryExpr -> condition
                else -> PsiTreeUtil.findChildOfType(condition, GoBinaryExpr::class.java)
            }
            
            if (binaryExpr != null) {
                val leftExpr = binaryExpr.left
                if (leftExpr is GoReferenceExpression) {
                    val varName = leftExpr.identifier?.text
                    if (varName != null && !isLikelyErrorVariable(varName)) {
                        MtlogLogger.info("Parent if is checking non-error variable '$varName', different logical context", element.project)
                        return true
                    }
                }
            }
        }
        
        return false
    }
}